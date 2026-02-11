package set

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas
	r.Target.Status.ReadyReplicas = 0
	r.Target.Status.ProgressingReplicas = 0
	r.Target.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		status := v2.EvaluateFirewallStatus(fw, c.c.GetCreateTimeout(), c.c.GetFirewallHealthTimeout())

		switch status.Result {
		case v2.FirewallStatusReady:
			r.Target.Status.ReadyReplicas++
			continue
		case v2.FirewallStatusProgressing:
			r.Target.Status.ProgressingReplicas++
			continue
		case v2.FirewallStatusUnhealthy, v2.FirewallStatusCreateTimeout, v2.FirewallStatusHealthTimeout:
			fallthrough
		default:
			r.Target.Status.UnhealthyReplicas++
			continue
		}
	}

	revision, err := controllers.Revision(r.Target)
	if err != nil {
		return err
	}
	r.Target.Status.ObservedRevision = revision

	return nil
}
