package deployment

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	err := c.ensureFirewallControllerRBAC(r)
	if err != nil {
		return err
	}

	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.Seed, nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	current, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if current == nil {
		r.Log.Info("no firewall set is present, creating a new one")

		_, err := c.createFirewallSet(r, 0)
		if err != nil {
			return err
		}

		return nil
	}

	switch s := r.Target.Spec.Strategy; s {
	case v2.StrategyRecreate:
		err = c.recreateStrategy(r, ownedSets, current)
	case v2.StrategyRollingUpdate:
		err = c.rollingUpdateStrategy(r, ownedSets, current)
	default:
		err = fmt.Errorf("unknown deployment strategy: %s", s)
	}

	statusErr := c.setStatus(r)

	if err != nil {
		return err
	}
	if statusErr != nil {
		return err
	}

	return nil
}

func (c *controller) createNextFirewallSet(r *controllers.Ctx[*v2.FirewallDeployment], current *v2.FirewallSet) (*v2.FirewallSet, error) {
	revision, err := controllers.NextRevision(current)
	if err != nil {
		return nil, err
	}

	return c.createFirewallSet(r, revision)
}

func (c *controller) createFirewallSet(r *controllers.Ctx[*v2.FirewallDeployment], revision int) (*v2.FirewallSet, error) {
	if lastCreation, ok := c.lastSetCreation[r.Target.Name]; ok && time.Since(lastCreation) < c.SafetyBackoff {
		// this is just for safety reasons to prevent mass-allocations
		r.Log.Info("backing off from firewall set creation as last creation is only seconds ago", "ago", time.Since(lastCreation).String())
		return nil, controllers.RequeueAfter(10*time.Second, "delaying firewall set creation")
	}

	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	userdata, err := c.createUserdata(r.Ctx)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s-%s", r.Target.Name, uuid.String()[:5])

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
		},
		Spec: v2.FirewallSetSpec{
			Replicas: r.Target.Spec.Replicas,
			Template: r.Target.Spec.Template,
			Userdata: userdata,
		},
	}

	err = c.Seed.Create(r.Ctx, set, &client.CreateOptions{})
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
	set.Spec.Replicas = r.Target.Spec.Replicas
	set.Spec.Template = r.Target.Spec.Template

	err := c.Seed.Update(r.Ctx, set)
	if err != nil {
		return fmt.Errorf("unable to update/sync firewall set: %w", err)
	}

	r.Log.Info("updated firewall set", "set-name", set.Name)

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "FirewallSetUpdated", fmt.Sprintf("Updated firewall set %q.", set.Name))
	r.Target.Status.Conditions.Set(cond)

	c.Recorder.Eventf(set, "Normal", "Update", "updated firewallset %s", set.Name)

	return nil
}

func (c *controller) isNewSetRequired(r *controllers.Ctx[*v2.FirewallDeployment], lastSet *v2.FirewallSet) (bool, error) {
	v, ok := lastSet.Annotations[v2.RollSetAnnotation]
	if ok {
		rollSet, err := strconv.ParseBool(v)
		if err == nil && rollSet {
			r.Log.Info("set roll initiated by annotation")
			return true, nil
		}
	}

	var (
		newS = &r.Target.Spec.Template.Spec
		oldS = &lastSet.Spec.Template.Spec
	)

	ok = sizeHasChanged(newS, oldS)
	if ok {
		r.Log.Info("firewall size has changed", "size", newS.Size)
		return ok, nil
	}

	ok, err := osImageHasChanged(r.Ctx, c.Metal, newS, oldS)
	if err != nil {
		return false, err
	}
	if ok {
		r.Log.Info("firewall image has changed", "image", newS.Image)
		return ok, nil
	}

	ok = networksHaveChanged(newS, oldS)
	if ok {
		r.Log.Info("firewall networks have changed", "networks", newS.Networks)
		return ok, nil
	}

	return false, nil
}

func sizeHasChanged(newS *v2.FirewallSpec, oldS *v2.FirewallSpec) bool {
	return newS.Size != oldS.Size
}

func osImageHasChanged(ctx context.Context, m metalgo.Client, newS *v2.FirewallSpec, oldS *v2.FirewallSpec) (bool, error) {
	if newS.Image != oldS.Image {
		image, err := m.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(newS.Image).WithContext(ctx), nil)
		if err != nil {
			return false, fmt.Errorf("latest firewall image not found:%s %w", newS.Image, err)
		}

		if image.Payload != nil && image.Payload.ID != nil && *image.Payload.ID != oldS.Image {
			return true, nil
		}
	}

	return false, nil
}

func networksHaveChanged(newS *v2.FirewallSpec, oldS *v2.FirewallSpec) bool {
	return !sets.NewString(oldS.Networks...).Equal(sets.NewString(newS.Networks...))
}
