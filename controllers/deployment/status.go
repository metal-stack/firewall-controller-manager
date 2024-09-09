package deployment

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallDeployment], ownedSets []*v2.FirewallSet) error {
	latestSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas

	if latestSet != nil {
		revision, err := controllers.Revision(latestSet)
		if err != nil {
			return err
		}
		r.Target.Status.ObservedRevision = revision
		r.Target.Status.ProgressingReplicas = latestSet.Status.ProgressingReplicas
		r.Target.Status.UnhealthyReplicas = latestSet.Status.UnhealthyReplicas
		r.Target.Status.ReadyReplicas = latestSet.Status.ReadyReplicas
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
