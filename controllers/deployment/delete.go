package deployment

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.Seed, nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	return c.deleteFirewallSets(r, ownedSets...)
}

func (c *controller) deleteFirewallSets(r *controllers.Ctx[*v2.FirewallDeployment], sets ...*v2.FirewallSet) error {
	for _, set := range sets {
		set := set

		if set.DeletionTimestamp != nil {
			r.Log.Info("deletion timestamp on firewall set already set", "firewall-name", set.Name)
			continue
		}

		err := c.Seed.Delete(r.Ctx, set)
		if err != nil {
			return err
		}

		r.Log.Info("set deletion timestamp on firewall set", "set-name", set.Name)

		c.Recorder.Eventf(set, "Normal", "Delete", "deleted firewallset %s", set.Name)
	}

	if len(sets) > 0 {
		return controllers.RequeueAfter(2*time.Second, "firewall sets are getting deleted, waiting")
	}

	return nil
}
