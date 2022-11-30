package deployment

import (
	"context"
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) setStatus(ctx context.Context, deploy *v2.FirewallDeployment) error {
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

	deploy.Status.TargetReplicas = deploy.Spec.Replicas

	if lastSet != nil {
		revision, err := controllers.Revision(lastSet)
		if err != nil {
			return err
		}
		deploy.Status.ObservedRevision = revision
		deploy.Status.ProgressingReplicas = lastSet.Status.ProgressingReplicas
		deploy.Status.UnhealthyReplicas = lastSet.Status.UnhealthyReplicas
		deploy.Status.ReadyReplicas = lastSet.Status.ReadyReplicas
	}

	if deploy.Status.ReadyReplicas >= deploy.Spec.Replicas {
		cond := v2.NewCondition(v2.FirewallDeplomentAvailable, v2.ConditionTrue, "MinimumReplicasAvailable", "Deployment has minimum availability.")
		deploy.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallDeplomentAvailable, v2.ConditionFalse, "MinimumReplicasUnavailable", "Deployment does not have minimum availability.")
		deploy.Status.Conditions.Set(cond)
	}

	return nil
}
