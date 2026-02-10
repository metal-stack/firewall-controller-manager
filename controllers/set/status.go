package set

import (
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

type status string

const (
	statusReady         status = "ready"
	statusProgressing   status = "progressing"
	statusUnhealthy     status = "unhealthy"
	statusHealthTimeout status = "health-timeout"
	statusCreateTimeout status = "create-timeout"
)

func (c *controller) evaluateFirewallConditions(fw *v2.Firewall) status {
	switch fw.Status.Phase {
	case v2.FirewallPhaseCreating, v2.FirewallPhaseCrashing:
		var (
			createTimeout = c.c.GetCreateTimeout()
			provisioned   = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallProvisioned)).Status == v2.ConditionTrue
		)

		if provisioned {
			return statusReady
		}

		if createTimeout > 0 {
			createTimeout := c.c.GetCreateTimeout()

			if ok := checkForTimeout(fw, v2.FirewallReady, createTimeout); ok {
				c.log.Info("create timeout exceeded, firewall not provisioned in time", "firewall-name", fw.Name, "timeout-after", createTimeout.String())
				return statusCreateTimeout
			}
		}

		return statusProgressing

	case v2.FirewallPhaseRunning:
		fallthrough

	default:
		var (
			created            = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
			ready              = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
			provisioned        = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallProvisioned)).Status == v2.ConditionTrue
			connected          = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Status == v2.ConditionTrue
			seedConnected      = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)).Status == v2.ConditionTrue
			distanceConfigured = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallDistanceConfigured)).Status == v2.ConditionTrue

			allConditionsMet = created && ready && provisioned && connected && seedConnected && distanceConfigured
		)

		if allConditionsMet {
			return statusReady
		}

		if provisioned {
			healthTimeout := c.c.GetFirewallHealthTimeout()

			switch {
			case !seedConnected:
				if ok := checkForTimeout(fw, v2.FirewallControllerSeedConnected, healthTimeout); ok {
					c.log.Info("health timeout exceeded, seed connection lost", "firewall-name", fw.Name, "timeout-after", healthTimeout.String())
					return statusHealthTimeout
				}

			case !connected:
				if ok := checkForTimeout(fw, v2.FirewallControllerConnected, healthTimeout); ok {
					c.log.Info("health timeout exceeded, firewall monitor not reconciled anymore by controller", "firewall-name", fw.Name, "timeout-after", healthTimeout.String())
					return statusHealthTimeout
				}

			case !ready:
				if ok := checkForTimeout(fw, v2.FirewallReady, healthTimeout); ok {
					c.log.Info("health timeout exceeded, firewall is not ready from perspective of the metal-api", "firewall-name", fw.Name, "timeout-after", healthTimeout.String())
					return statusHealthTimeout
				}
			}
		}

		return statusUnhealthy
	}
}

func (c *controller) setStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	r.Target.Status.TargetReplicas = r.Target.Spec.Replicas
	r.Target.Status.ReadyReplicas = 0
	r.Target.Status.ProgressingReplicas = 0
	r.Target.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		statusReport := c.evaluateFirewallConditions(fw)

		switch statusReport {
		case statusReady:
			r.Target.Status.ReadyReplicas++
			continue
		case statusProgressing:
			r.Target.Status.ProgressingReplicas++
			continue
		case statusUnhealthy, statusCreateTimeout, statusHealthTimeout:
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

func checkForTimeout(fw *v2.Firewall, condition v2.ConditionType, timeout time.Duration) bool {
	if timeout == 0 {
		return false
	}

	cond := pointer.SafeDeref(fw.Status.Conditions.Get(condition))

	return time.Since(cond.LastTransitionTime.Time) > timeout
}
