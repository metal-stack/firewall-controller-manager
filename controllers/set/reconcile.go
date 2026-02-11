package set

import (
	"fmt"
	"maps"

	"github.com/Masterminds/semver/v3"
	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, orphaned, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), r.Target.Spec.Selector, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	adoptions, err := c.adoptFirewalls(r, orphaned)
	if err != nil {
		return fmt.Errorf("error when trying to adopt firewalls: %w", err)
	}

	ownedFirewalls = append(ownedFirewalls, adoptions...)

	// we sort for distance orchestration and for scale down deletion
	// - the most important firewall will get the shortest distance to attract all the traffic
	// - the least important at the end of the slice can be popped off for deletion on scale down
	v2.SortFirewallsByImportance(ownedFirewalls)

	for i, ownedFw := range ownedFirewalls {
		fw := &v2.Firewall{}
		err := c.c.GetSeedClient().Get(r.Ctx, client.ObjectKeyFromObject(ownedFw), fw)
		if err != nil {
			return fmt.Errorf("error fetching firewall: %w", err)
		}

		fw.Spec = r.Target.Spec.Template.Spec

		// stagger firewall replicas to achieve active/standby behavior
		//
		// we give the most important firewall the shortest distance within a set
		// this firewall attracts the traffic within the replica set
		//
		// the second most important firewall gets a longer distance
		// such that it takes over the traffic in case the first replica dies
		// (first-level backup)
		//
		// the rest of the firewalls get an even higher distance
		// (third-level backup)
		// one of them moves up on next set sync after deletion of the "active" replica
		var distance v2.FirewallDistance
		switch i {
		case 0:
			distance = r.Target.Spec.Distance + 0
		case 1:
			distance = r.Target.Spec.Distance + 1
		default:
			distance = r.Target.Spec.Distance + 2
		}

		if distance > v2.FirewallLongestDistance {
			distance = v2.FirewallLongestDistance
		}

		fw.Distance = distance

		err = c.c.GetSeedClient().Update(r.Ctx, fw, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating firewall spec: %w", err)
		}

		r.Log.Info("updated/synced firewall", "name", fw.Name, "distance", distance)
	}

	currentAmount := len(ownedFirewalls)

	if currentAmount < r.Target.Spec.Replicas {
		r.Log.Info("scale up", "current", currentAmount, "want", r.Target.Spec.Replicas)

		for i := currentAmount; i < r.Target.Spec.Replicas; i++ {
			fw, err := c.createFirewall(r)
			if err != nil {
				return err
			}

			r.Log.Info("firewall created", "firewall-name", fw.Name)

			c.recorder.Eventf(r.Target, nil, "Normal", "Create", "created firewall %s", fw.Name)

			ownedFirewalls = append(ownedFirewalls, fw)
		}
	}

	if currentAmount > r.Target.Spec.Replicas {
		r.Log.Info("scale down", "current", currentAmount, "want", r.Target.Spec.Replicas)

		for i := r.Target.Spec.Replicas; i < currentAmount; i++ {
			var fw *v2.Firewall
			fw, ownedFirewalls = pop(ownedFirewalls)

			err := c.deleteFirewalls(r, fw)
			if err != nil {
				return err
			}
		}
	}

	deletedFws, err := c.deleteIfUnhealthyOrTimeout(r, ownedFirewalls...)
	if err != nil {
		return err
	}

	ownedFirewalls = controllers.Except(ownedFirewalls, deletedFws...)

	err = c.setStatus(r, ownedFirewalls)
	if err != nil {
		return err
	}

	return nil
}

func pop[E any](slice []E) (E, []E) {
	// stolen from golang slice tricks
	return slice[len(slice)-1], slice[:len(slice)-1]
}

func (c *controller) createFirewall(r *controllers.Ctx[*v2.FirewallSet]) (*v2.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := r.Target.Namespace
	name := fmt.Sprintf("%s-firewall-%s", clusterName, uuid.String()[:5])

	meta := r.Target.Spec.Template.ObjectMeta.DeepCopy()
	meta.Name = name
	meta.Namespace = r.Target.Namespace
	meta.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(r.Target, v2.GroupVersion.WithKind("FirewallSet")),
	}

	// inheriting labels from the firewall set to the firewall
	maps.Copy(meta.Labels, r.Target.Labels)

	if v, err := semver.NewVersion(r.Target.Spec.Template.Spec.ControllerVersion); err == nil && v.LessThan(semver.MustParse("v2.0.0")) {
		if meta.Annotations == nil {
			meta.Annotations = map[string]string{}
		}
		meta.Annotations[v2.FirewallNoControllerConnectionAnnotation] = "true"
	}

	if r.Target.Annotations != nil {
		if val, ok := r.Target.Annotations[v2.FirewallNoControllerConnectionAnnotation]; ok {
			if meta.Annotations == nil {
				meta.Annotations = map[string]string{}
			}
			meta.Annotations[v2.FirewallNoControllerConnectionAnnotation] = val
		}
	}

	fw := &v2.Firewall{
		ObjectMeta: *meta,
		Spec:       r.Target.Spec.Template.Spec,
		Distance:   r.Target.Spec.Distance,
	}

	err = c.c.GetSeedClient().Create(r.Ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}

func (c *controller) adoptFirewalls(r *controllers.Ctx[*v2.FirewallSet], fws []*v2.Firewall) ([]*v2.Firewall, error) {
	var adoptions []*v2.Firewall

	for _, fw := range fws {
		ok, err := c.adoptFirewall(r, fw)
		if err != nil {
			return nil, err
		}

		if ok {
			r.Log.Info("adopted firewall", "firewall-name", fw.Name)
			adoptions = append(adoptions, fw)
		}
	}

	return adoptions, nil
}

func (c *controller) adoptFirewall(r *controllers.Ctx[*v2.FirewallSet], fw *v2.Firewall) (adopted bool, err error) {
	if fw.DeletionTimestamp != nil {
		// don't adopt in deletion firewalls
		return false, nil
	}

	ref := metav1.GetControllerOf(fw)
	if ref != nil && ref.UID != r.Target.UID {
		// the firewall belongs to some other controller
		return false, nil
	}

	fw.OwnerReferences = append(fw.OwnerReferences, *metav1.NewControllerRef(r.Target, v2.GroupVersion.WithKind("FirewallSet")))

	err = c.c.GetSeedClient().Update(r.Ctx, fw)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}

	return true, nil
}
