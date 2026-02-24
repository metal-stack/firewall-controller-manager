package set

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"

	corev1 "k8s.io/api/core/v1"
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
		if fw.DeletionTimestamp != nil {
			r.Log.Info("deletion timestamp on firewall already set", "firewall-name", fw.Name)
			continue
		}

		err := c.c.GetSeedClient().Delete(r.Ctx, fw)
		if err != nil {
			return err
		}

		r.Log.Info("set deletion timestamp on firewall", "firewall-name", fw.Name)

		c.recorder.Eventf(fw, nil, corev1.EventTypeNormal, "Delete", "deleting firewall", "deleted firewall %s", fw.Name)
	}

	if len(fws) > 0 {
		return controllers.RequeueAfter(2*time.Second, "firewalls are getting deleted, waiting")
	}

	return nil
}
