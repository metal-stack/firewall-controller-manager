package set

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, err := controllers.GetOwnedResources(r.Ctx, c.Seed, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	for _, fw := range ownedFirewalls {
		fw := fw

		err = c.Seed.Delete(r.Ctx, fw, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		c.Recorder.Eventf(fw, "Normal", "Delete", "deleted firewall %s", fw.Name)
	}

	return nil
}
