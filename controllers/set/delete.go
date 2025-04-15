package set

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
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
		status := c.evaluateFirewallConditions(fw)

		switch {
		case status.CreateTimeout || status.HealthTimeout:
			r.Log.Info("firewall health or creation timeout exceeded, deleting from set", "firewall-name", fw.Name)

			err := c.deleteFirewalls(r, fw)
			if err != nil {
				return nil, err
			}

			result = append(result, fw)
		}

	}
	return result, nil
}
