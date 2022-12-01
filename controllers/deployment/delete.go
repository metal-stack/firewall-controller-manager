package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	ownedSets, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	return c.deleteFirewallSets(r, ownedSets)
}

func (c *controller) deleteFirewallSets(r *controllers.Ctx[*v2.FirewallDeployment], sets []*v2.FirewallSet) error {
	for _, set := range sets {
		set := set

		err := c.Seed.Delete(r.Ctx, set, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		r.Log.Info("deleted firewall set", "name", set.Name)

		c.Recorder.Eventf(set, "Normal", "Delete", "deleted firewallset %s", set.Name)
	}

	return nil
}
