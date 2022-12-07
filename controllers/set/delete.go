package set

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, _, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target.Spec.Selector, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	return c.deleteFirewalls(r, ownedFirewalls...)
}

func (c *controller) deleteFirewalls(r *controllers.Ctx[*v2.FirewallSet], fws ...*v2.Firewall) error {
	for _, fw := range fws {
		fw := fw

		err := c.Seed.Delete(r.Ctx, fw)
		if err != nil {
			return err
		}

		r.Log.Info("set deletion timestamp on firewall", "firewall-name", fw.Name)

		c.Recorder.Eventf(fw, "Normal", "Delete", "deleted firewall %s", fw.Name)
	}

	if len(fws) > 0 {
		return controllers.RequeueAfter(2*time.Second, "firewalls are getting deleted, waiting")
	}

	return nil
}

func (c *controller) deleteAfterTimeout(r *controllers.Ctx[*v2.FirewallSet], fws ...*v2.Firewall) ([]*v2.Firewall, error) {
	var result []*v2.Firewall

	for _, fw := range fws {
		fw := fw

		if fw.Status.Phase != v2.FirewallPhaseCreating {
			continue
		}

		connected := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue

		if !connected && time.Since(fw.CreationTimestamp.Time) > c.CreateTimeout {
			r.Log.Info("firewall not getting ready, deleting from set", "firewall-name", fw.Name)

			err := c.deleteFirewalls(r, fw)
			if err != nil {
				return nil, err
			}

			result = append(result, fw)
		}
	}

	return result, nil
}

// deletePhysicalOrphans checks in the backend if there are firewall entities that belong to the controller
// event though there is no corresponding firewall resource managed by this controller.
//
// such firewalls will be deleted in the backend.
func (c *controller) deletePhysicalOrphans(r *controllers.Ctx[*v2.FirewallSet]) error {
	resp, err := c.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: r.Target.Spec.Template.Spec.Project,
		Tags:              []string{c.ClusterTag, controllers.FirewallSetTag(r.Target.Name)},
	}).WithContext(r.Ctx), nil)
	if err != nil {
		r.Log.Error(err, "unable to retrieve firewalls for orphan checking, backing off...")
		return controllers.RequeueAfter(10*time.Second, "backing off")
	}

	if len(resp.Payload) == 0 {
		return nil
	}

	fws := &v2.FirewallList{}
	err = c.Seed.List(r.Ctx, fws, client.InNamespace(c.Namespace))
	if err != nil {
		return err
	}

	ownedFirewalls, _, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target.Spec.Selector, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
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

		r.Log.Info("found physical orphan firewall, deleting", "firewall-name", *fw.Allocation.Name, "id", *fw.ID, "non-orphans", existingNames)

		_, err = c.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*fw.ID), nil)
		if err != nil {
			return fmt.Errorf("error deleting orphaned firewall: %w", err)
		}

		c.Recorder.Eventf(r.Target, "Normal", "Delete", "deleted orphaned firewall %s id %s", *fw.Allocation.Name, *fw.ID)
	}

	return nil
}
