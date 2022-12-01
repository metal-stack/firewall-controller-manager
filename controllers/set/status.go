package set

import (
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas

	r.Target.Status.ReadyReplicas = 0
	r.Target.Status.ProgressingReplicas = 0
	r.Target.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		var (
			fw = fw

			created = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
			ready   = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
			// FIXME: enable back in real environment:
			// connected = fw.Status.Conditions.Get(v2.FirewallControllerConnected).Status == v2.ConditionTrue
		)

		if created && ready {
			r.Target.Status.ReadyReplicas++
			continue
		}

		if created && time.Since(pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time) < c.FirewallHealthTimeout {
			r.Target.Status.ProgressingReplicas++
			continue
		}

		r.Target.Status.UnhealthyReplicas++
	}

	revision, err := controllers.Revision(r.Target)
	if err != nil {
		return err
	}
	r.Target.Status.ObservedRevision = revision

	return nil
}
