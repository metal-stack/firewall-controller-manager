package deployment

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	return c.deleteFirewallSets(r, ownedSets...)
}

func (c *controller) deleteFirewallSets(r *controllers.Ctx[*v2.FirewallDeployment], sets ...*v2.FirewallSet) error {
	for _, set := range sets {
		if set.DeletionTimestamp != nil {
			r.Log.Info("deletion timestamp on firewall set already set", "firewall-name", set.Name)
			continue
		}

		err := c.c.GetSeedClient().Delete(r.Ctx, set)
		if err != nil {
			return err
		}

		r.Log.Info("set deletion timestamp on firewall set", "set-name", set.Name)

		c.recorder.Eventf(set, nil, corev1.EventTypeNormal, "Delete", "deleting set", "deleted firewall set %s", set.Name)
	}

	if len(sets) > 0 {
		return controllers.RequeueAfter(2*time.Second, "firewall sets are getting deleted, waiting")
	}

	return nil
}
