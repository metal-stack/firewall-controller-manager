package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	ownedSets, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), nil, r.Target, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas

	if lastSet != nil {
		revision, err := controllers.Revision(lastSet)
		if err != nil {
			return err
		}
		r.Target.Status.ObservedRevision = revision
		r.Target.Status.ProgressingReplicas = lastSet.Status.ProgressingReplicas
		r.Target.Status.UnhealthyReplicas = lastSet.Status.UnhealthyReplicas
		r.Target.Status.ReadyReplicas = lastSet.Status.ReadyReplicas
	}

	if r.Target.Status.ReadyReplicas >= r.Target.Spec.Replicas {
		cond := v2.NewCondition(v2.FirewallDeplomentAvailable, v2.ConditionTrue, "MinimumReplicasAvailable", "Deployment has minimum availability.")
		r.Target.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallDeplomentAvailable, v2.ConditionFalse, "MinimumReplicasUnavailable", "Deployment does not have minimum availability.")
		r.Target.Status.Conditions.Set(cond)
	}

	return nil
}
