package set

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	for _, fw := range ownedFirewalls {
		fw.Spec = r.Target.Spec.Template

		err := c.Seed.Update(r.Ctx, fw, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating firewall spec: %w", err)
		}
	}

	currentAmount := len(ownedFirewalls)

	if currentAmount < r.Target.Spec.Replicas {
		for i := currentAmount; i < r.Target.Spec.Replicas; i++ {
			fw, err := c.createFirewall(r)
			if err != nil {
				return err
			}

			r.Log.Info("firewall created", "firewall-name", fw.Name)

			c.Recorder.Eventf(r.Target, "Normal", "Create", "created firewall %s", fw.Name)

			ownedFirewalls = append(ownedFirewalls, fw)
		}
	}

	if currentAmount > r.Target.Spec.Replicas {
		// TODO: this section needs testing
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

			ownedFirewalls = controllers.Except(ownedFirewalls, fw)
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

	return c.checkOrphans(r)
}

func (c *controller) createFirewall(r *controllers.Ctx[*v2.FirewallSet]) (*v2.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := r.Target.Namespace
	name := fmt.Sprintf("%s-firewall-%s", clusterName, uuid.String()[:5])

	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.Target.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(r.Target, v2.GroupVersion.WithKind("FirewallSet")),
			},
		},
		Spec:     r.Target.Spec.Template,
		Userdata: r.Target.Userdata,
	}

	err = c.Seed.Create(r.Ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}

func (c *controller) checkOrphans(r *controllers.Ctx[*v2.FirewallSet]) error {
	resp, err := c.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: r.Target.Spec.Template.Project,
		Tags:              []string{c.ClusterTag, controllers.FirewallSetTag(r.Target.Name)},
	}).WithContext(r.Ctx), nil)
	if err != nil {
		return err
	}

	if len(resp.Payload) == 0 {
		return nil
	}

	fws := &v2.FirewallList{}
	err = c.Seed.List(r.Ctx, fws, client.InNamespace(c.Namespace))
	if err != nil {
		return err
	}

	ownedFirewalls, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	existingNames := map[string]bool{}
	for _, fw := range ownedFirewalls {
		existingNames[fw.Name] = true
	}

	for _, fw := range resp.Payload {
		if fw.Allocation == nil || fw.Allocation.Name == nil {
			continue
		}
		if _, ok := existingNames[*fw.Allocation.Name]; ok {
			continue
		}

		r.Log.Info("found orphan firewall, deleting orphan", "firewall-name", *fw.Allocation.Name, "id", *fw.ID, "non-orphans", existingNames)

		_, err = c.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*fw.ID), nil)
		if err != nil {
			return fmt.Errorf("error deleting orphaned firewall: %w", err)
		}

		c.Recorder.Eventf(r.Target, "Normal", "Delete", "deleted orphaned firewall %s id %s", *fw.Allocation.Name, *fw.ID)
	}

	return nil
}

func pop[E any](slice []E) (E, []E) {
	// stolen from golang slice tricks
	return slice[len(slice)-1], slice[:len(slice)-1]
}
