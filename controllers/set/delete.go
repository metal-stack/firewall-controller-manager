package set

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
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

		err := c.Seed.Delete(r.Ctx, fw, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		r.Log.Info("deleted firewall", "firewall-name", fw.Name)

		c.Recorder.Eventf(fw, "Normal", "Delete", "deleted firewall %s", fw.Name)
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

		// FIXME: enable back in real environment:
		// connected := pointer.SafeDeref(r.Target.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
		ready := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue

		if !ready && time.Since(fw.CreationTimestamp.Time) > c.createTimeout {
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
