package set

import (
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

type FirewallConditionStatus struct {
	IsReady, IsProgressing, IsUnhealthy bool
}

func evaluateFirewallConditions(fw *v2.Firewall, healthTimeout time.Duration) FirewallConditionStatus {
	created := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
	ready := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
	connected := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
	seedConnected := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)).Status == v2.ConditionTrue
	distance := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallDistanceConfigured)).Status == v2.ConditionTrue

	allConditionsMet := created && ready && connected && seedConnected && distance
	allocationTimeExceeded := created && time.Since(pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time) < healthTimeout
	unhealthyTimeExceeded := created && time.Since(pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time) > healthTimeout

	return FirewallConditionStatus{
		IsReady:       allConditionsMet,
		IsProgressing: allocationTimeExceeded,
		IsUnhealthy:   unhealthyTimeExceeded,
	}
}

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas
	r.Target.Status.ReadyReplicas = 0
	r.Target.Status.ProgressingReplicas = 0
	r.Target.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		statusReport := evaluateFirewallConditions(fw, c.c.GetFirewallHealthTimeout())
		if statusReport.IsReady {
			r.Target.Status.ReadyReplicas++
			continue
		}
		if statusReport.IsProgressing {
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
