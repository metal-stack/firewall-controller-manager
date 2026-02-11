package set

import (
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

type firewallConditionStatus struct {
	IsReady       bool
	CreateTimeout bool
	HealthTimeout bool
}

func (c *controller) evaluateFirewallConditions(fw *v2.Firewall) firewallConditionStatus {
	var (
		unhealthyTimeout  = c.c.GetFirewallHealthTimeout()
		allocationTimeout = c.c.GetCreateTimeout()

		created            = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
		ready              = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
		connected          = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
		seedConnected      = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)).Status == v2.ConditionTrue
		distanceConfigured = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallDistanceConfigured)).Status == v2.ConditionTrue
		allConditionsMet   = created && ready && connected && seedConnected && distanceConfigured

		seedUpdatedTime    = pointer.SafeDeref(fw.Status.ControllerStatus).SeedUpdated.Time
		timeSinceReconcile = time.Since(seedUpdatedTime)
		allocationTime     = pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time
	)

	if allConditionsMet {
		return firewallConditionStatus{IsReady: true}
	}

	// duration after which a firewall in the creation phase will be recreated, exceeded
	if allocationTimeout > 0 && fw.Status.Phase == v2.FirewallPhaseCreating && !allocationTime.IsZero() {
		if time.Since(allocationTime) > allocationTimeout {
			c.log.Info("create timeout exceeded", "firewall-name", fw.Name, "allocated-at", allocationTime.String(), "timeout-after", allocationTimeout.String())
			return firewallConditionStatus{CreateTimeout: true}
		}
	}
	// Only apply health timeout once we have a non-zero seed reconcile timestamp.
	if (!ready || !seedConnected || !connected) && unhealthyTimeout > 0 && created && !seedUpdatedTime.IsZero() && timeSinceReconcile > unhealthyTimeout {
		c.log.Info("health timeout exceeded", "firewall-name", fw.Name, "last-reconciled-at", seedUpdatedTime.String(), "timeout-after", unhealthyTimeout.String())
		return firewallConditionStatus{HealthTimeout: true}
	}
	// Firewall was healthy at one point (all conditions were met), but then one of the monitor conditions
	// degraded so the firewall is unhealthy. Only check monitor conditions (connected, seedConnected, distanceConfigured)
	// because the ready condition degradation is already handled by the time-based health timeout above.
	wasHealthy := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallHealthy)).Status == v2.ConditionTrue
	monitorConditionsDegraded := !connected || !seedConnected || !distanceConfigured
	if monitorConditionsDegraded && wasHealthy && unhealthyTimeout > 0 {
		c.log.Info("firewall monitor conditions degraded", "firewall-name", fw.Name)
		return firewallConditionStatus{HealthTimeout: true}
	}
	//if everything returns false, it is progressing
	return firewallConditionStatus{
		IsReady:       allConditionsMet,
		CreateTimeout: false,
		HealthTimeout: false,
	}
}

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas
	r.Target.Status.ReadyReplicas = 0
	r.Target.Status.ProgressingReplicas = 0
	r.Target.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		statusReport := c.evaluateFirewallConditions(fw)

		switch {
		case statusReport.IsReady:
			r.Target.Status.ReadyReplicas++
			continue

		case statusReport.CreateTimeout || statusReport.HealthTimeout:
			r.Target.Status.UnhealthyReplicas++
			continue
		}

		r.Target.Status.ProgressingReplicas++
	}

	revision, err := controllers.Revision(r.Target)
	if err != nil {
		return err
	}
	r.Target.Status.ObservedRevision = revision

	return nil
}
