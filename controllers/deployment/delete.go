package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment) error {
	ownedSets, err := controllers.GetOwnedResources(ctx, c.Seed, deploy, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	for _, s := range ownedSets {
		s := s

		log.Info("deleting firewall set", "name", s.Name)

		err = c.Seed.Delete(ctx, s, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		c.Recorder.Event(s, "Normal", "Delete", fmt.Sprintf("deleted firewallset %s", s.Name))
	}

	return nil
}
