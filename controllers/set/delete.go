package set

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), r.Target.Spec.Selector, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
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

		if fw.DeletionTimestamp != nil {
			r.Log.Info("deletion timestamp on firewall already set", "firewall-name", fw.Name)
			continue
		}

		err := c.c.GetSeedClient().Delete(r.Ctx, fw)
		if err != nil {
			return err
		}

		r.Log.Info("set deletion timestamp on firewall", "firewall-name", fw.Name)

		c.recorder.Eventf(fw, "Normal", "Delete", "deleted firewall %s", fw.Name)
	}

	if len(fws) > 0 {
		return controllers.RequeueAfter(2*time.Second, "firewalls are getting deleted, waiting")
	}

	return nil
}
func (c *controller) deleteIfUnhealthyOrTimeout(r *controllers.Ctx[*v2.FirewallSet], fws ...*v2.Firewall) ([]*v2.Firewall, error) {
	var result []*v2.Firewall

	for _, fw := range fws {
		if c.isFirewallUnhealthy(fw) {
			r.Log.Info("unhealthy firewall not recovering, deleting from set", "firewall-name", fw.Name)
			err := c.deleteFirewalls(r, fw)
			if err != nil {
				return nil, err
			}
			result = append(result, fw)
			continue
		}

		if fw.Status.Phase != v2.FirewallPhaseCreating {
			continue
		}
		connected := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
		if !connected && time.Since(fw.CreationTimestamp.Time) > c.c.GetCreateTimeout() {
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

func (c *controller) isFirewallUnhealthy(fw *v2.Firewall) bool {
	statusReport := evaluateFirewallConditions(fw, c.c.GetFirewallHealthTimeout())
	return statusReport.IsUnhealthy
}
