package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) Status(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment) error {
	ownedSets, err := controllers.GetOwnedResources(ctx, c.Seed, deploy, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	status := v2.FirewallDeploymentStatus{}

	if lastSet != nil {
		status.ProgressingReplicas = lastSet.Status.ProgressingReplicas
		status.UnhealthyReplicas = lastSet.Status.UnhealthyReplicas
		status.ReadyReplicas = lastSet.Status.ReadyReplicas
	}

	deploy.Status = status

	return nil
}
