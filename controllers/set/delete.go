package set

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, set *v2.FirewallSet) error {
	ownedFirewalls, err := controllers.GetOwnedResources(ctx, c.Seed, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	for _, fw := range ownedFirewalls {
		fw := fw

		err = c.Seed.Delete(ctx, fw, &client.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
