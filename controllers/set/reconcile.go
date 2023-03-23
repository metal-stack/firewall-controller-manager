package set

import (
	"fmt"
	"sort"

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

	for _, fw := range ownedFirewalls {
		fw.Spec = r.Target.Spec.Template.Spec

		err := c.c.GetSeedClient().Update(r.Ctx, fw, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating firewall spec: %w", err)
		}
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

			c.recorder.Eventf(r.Target, "Normal", "Create", "created firewall %s", fw.Name)

			ownedFirewalls = append(ownedFirewalls, fw)
		}
	}

	if currentAmount > r.Target.Spec.Replicas {
		r.Log.Info("scale down", "current", currentAmount, "want", r.Target.Spec.Replicas)

		sort.Slice(ownedFirewalls, func(i, j int) bool {
			// put the oldest at the end of the slice, we will then pop them off
			return !ownedFirewalls[i].CreationTimestamp.Before(&ownedFirewalls[j].CreationTimestamp)
		})

		for i := r.Target.Spec.Replicas; i < currentAmount; i++ {
			var fw *v2.Firewall
			fw, ownedFirewalls = pop(ownedFirewalls)

			err := c.deleteFirewalls(r, fw)
			if err != nil {
				return err
			}
		}
	}

	deletedFws, err := c.deleteAfterTimeout(r, ownedFirewalls...)
	if err != nil {
		return err
	}

	ownedFirewalls = controllers.Except(ownedFirewalls, deletedFws...)

	err = c.setStatus(r, ownedFirewalls)
	if err != nil {
		return err
	}

	return c.deletePhysicalOrphans(r)
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

	for k, v := range r.Target.Labels {
		// inheriting labels from the firewall set to the firewall
		meta.Labels[k] = v
	}

	fw := &v2.Firewall{
		ObjectMeta: *meta,
		Spec:       r.Target.Spec.Template.Spec,
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
		fw := fw

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
