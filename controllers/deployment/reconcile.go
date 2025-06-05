package deployment

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	err := c.ensureFirewallControllerRBAC(r)
	if err != nil {
		return err
	}

	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	latestSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if latestSet == nil {
		r.Log.Info("no firewall set is present, creating a new one")

		_, err := c.createFirewallSet(r, 0, nil)
		if err != nil {
			return err
		}

		return nil
	}

	var reconcileErr error
	switch s := r.Target.Spec.Strategy; s {
	case v2.StrategyRecreate:
		reconcileErr = c.recreateStrategy(r, ownedSets, latestSet)
	case v2.StrategyRollingUpdate:
		reconcileErr = c.rollingUpdateStrategy(r, ownedSets, latestSet)
	default:
		reconcileErr = fmt.Errorf("unknown deployment strategy: %s", s)
	}

	// refetch sets after possible creation
	// we want to update the set status before a possible return due to reconciliation having failed
	ownedSets, _, err = controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	err = c.setStatus(r, ownedSets)
	if err != nil {
		return err
	}

	if reconcileErr != nil {
		return reconcileErr
	}

	// we are done with the update, give the set the shortest distance if this is not already the case
	if latestSet.Status.ReadyReplicas == latestSet.Spec.Replicas && latestSet.Spec.Distance != v2.FirewallShortestDistance {
		latestSet.Spec.Distance = v2.FirewallShortestDistance

		err := c.c.GetSeedClient().Update(r.Ctx, latestSet)
		if err != nil {
			return fmt.Errorf("unable to swap latest set distance to %d: %w", v2.FirewallShortestDistance, err)
		}

		r.Log.Info("swapped latest set to shortest distance", "distance", v2.FirewallShortestDistance)
	}

	infrastructureName, ok := extractInfrastructureNameFromSeedNamespace(r.Target.Namespace)
	if ok {
		var ownedFirewalls []*v2.Firewall
		for _, set := range ownedSets {
			fws, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
				return fl.GetItems()
			})
			if err != nil {
				return fmt.Errorf("unable to get owned firewalls: %w", err)
			}

			ownedFirewalls = append(ownedFirewalls, fws...)
		}

		err = c.updateInfrastructureStatus(r, infrastructureName, ownedFirewalls)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) createNextFirewallSet(r *controllers.Ctx[*v2.FirewallDeployment], set *v2.FirewallSet, ows *setOverrides) (*v2.FirewallSet, error) {
	revision, err := controllers.NextRevision(set)
	if err != nil {
		return nil, err
	}

	return c.createFirewallSet(r, revision, ows)
}

type setOverrides struct {
	// override default distance (shortest distance)
	distance *v2.FirewallDistance
	// override default replicas (inherited from set spec)
	replicas *int
}

func (c *controller) createFirewallSet(r *controllers.Ctx[*v2.FirewallDeployment], revision int, ows *setOverrides) (*v2.FirewallSet, error) {
	if lastCreation, ok := c.lastSetCreation[r.Target.Name]; ok && time.Since(lastCreation) < c.c.GetSafetyBackoff() {
		// this is just for safety reasons to prevent mass-allocations
		r.Log.Info("backing off from firewall set creation as last creation is only seconds ago", "ago", time.Since(lastCreation).String())
		return nil, controllers.RequeueAfter(10*time.Second, "delaying firewall set creation")
	}

	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	var (
		name     = fmt.Sprintf("%s-%s", r.Target.Name, uuid.String()[:5])
		distance = v2.FirewallShortestDistance
		replicas = r.Target.Spec.Replicas
	)

	if ows != nil && ows.distance != nil {
		distance = *ows.distance
	}
	if ows != nil && ows.replicas != nil {
		replicas = *ows.replicas
	}

	set := &v2.FirewallSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.Target.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.Target, v2.GroupVersion.WithKind("FirewallDeployment")),
			},
			Annotations: map[string]string{
				v2.RevisionAnnotation: strconv.Itoa(revision),
			},
			Labels: r.Target.Labels,
		},
		Spec: v2.FirewallSetSpec{
			Replicas: replicas,
			Template: r.Target.Spec.Template,
			Distance: distance,
		},
	}

	if r.Target.Annotations != nil {
		if val, ok := r.Target.Annotations[v2.FirewallNoControllerConnectionAnnotation]; ok {
			set.Annotations[v2.FirewallNoControllerConnectionAnnotation] = val
		}
	}

	err = c.c.GetSeedClient().Create(r.Ctx, set, &client.CreateOptions{})
	if err != nil {
		cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "FirewallSetCreateError", fmt.Sprintf("Error creating firewall set: %s.", err))
		r.Target.Status.Conditions.Set(cond)

		return nil, fmt.Errorf("unable to create firewall set: %w", err)
	}

	r.Log.Info("created new firewall set", "set-name", set.Name)

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetCreated", fmt.Sprintf("Created new firewall set %q.", set.Name))
	r.Target.Status.Conditions.Set(cond)

	c.lastSetCreation[r.Target.Name] = time.Now()

	return set, nil
}

func (c *controller) syncFirewallSet(r *controllers.Ctx[*v2.FirewallDeployment], set *v2.FirewallSet) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		refetched := &v2.FirewallSet{}
		err := c.c.GetSeedClient().Get(r.Ctx, client.ObjectKeyFromObject(set), refetched)
		if err != nil {
			return fmt.Errorf("unable re-fetch firewall set: %w", err)
		}

		refetched.Spec.Replicas = r.Target.Spec.Replicas
		refetched.Spec.Template = r.Target.Spec.Template

		err = c.c.GetSeedClient().Update(r.Ctx, refetched)
		if err != nil {
			return fmt.Errorf("unable to update/sync firewall set: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	r.Log.Info("updated firewall set", "set-name", set.Name)

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "FirewallSetUpdated", fmt.Sprintf("Updated firewall set %q.", set.Name))
	r.Target.Status.Conditions.Set(cond)

	c.recorder.Eventf(set, "Normal", "Update", "updated firewallset %s", set.Name)

	return nil
}

func (c *controller) isNewSetRequired(r *controllers.Ctx[*v2.FirewallDeployment], latestSet *v2.FirewallSet) bool {
	if v2.IsAnnotationTrue(latestSet, v2.RollSetAnnotation) {
		r.Log.Info("set roll initiated by annotation")
		return true
	}

	var (
		newS = &r.Target.Spec.Template.Spec
		oldS = &latestSet.Spec.Template.Spec
	)

	if newS.Size != oldS.Size {
		r.Log.Info("firewall size has changed", "size", newS.Size)
		return true
	}

	if newS.Image != oldS.Image {
		r.Log.Info("firewall image has changed", "image", newS.Image)
		return true
	}

	if !sets.NewString(oldS.Networks...).Equal(sets.NewString(newS.Networks...)) {
		r.Log.Info("firewall networks have changed", "networks", newS.Networks)
		return true
	}

	// TODO: improve and write tests
	if !cmp.Equal(oldS.InitialRuleSet, newS.InitialRuleSet) {
		r.Log.Info("firewall initial rule set have changed")
		return true
	}

	return false
}
