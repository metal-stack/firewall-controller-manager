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
	unhealthyTimeout := c.c.GetFirewallHealthTimeout()
	allocationTimeout := c.c.GetCreateTimeout()

	var (
		created            = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
		ready              = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
		connected          = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
		seedConnected      = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)).Status == v2.ConditionTrue
		distanceConfigured = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallDistanceConfigured)).Status == v2.ConditionTrue
		allConditionsMet   = created && ready && connected && seedConnected && distanceConfigured
	)

	seedUpdatedTime := pointer.SafeDeref(fw.Status.ControllerStatus).SeedUpdated.Time
	timeSinceReconcile := time.Since(seedUpdatedTime)
	allocationTime := pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time

	if allConditionsMet {
		return firewallConditionStatus{IsReady: true}
	}

	// duration after which a firewall in the creation phase will be recreated, exceeded
	if allocationTimeout != 0 && fw.Status.Phase == v2.FirewallPhaseCreating && !allocationTime.IsZero() {
		if time.Since(allocationTime) > allocationTimeout {
			c.log.Info("create timeout reached")
			return firewallConditionStatus{CreateTimeout: true}
		}
	}
	if seedConnected && unhealthyTimeout != 0 && created && timeSinceReconcile > unhealthyTimeout {
		c.log.Info("health timeout exceeded", "firewall-name", fw.Name, "last-reconciled-at", seedUpdatedTime.String(), "timeout-after", unhealthyTimeout.String())
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
