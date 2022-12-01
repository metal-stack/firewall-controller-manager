package deployment

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

// rollingUpdateStrategy first creates a new set and deletes the old one's when the new one becomes ready
func (c *controller) rollingUpdateStrategy(r *controllers.Ctx[*v2.FirewallDeployment], ownedSets []*v2.FirewallSet, current *v2.FirewallSet) error {
	newSetRequired, err := c.isNewSetRequired(r, current)
	if err != nil {
		return err
	}

	if newSetRequired {
		r.Log.Info("significant changes detected in the spec, creating new firewall set")

		newSet, err := c.createNextFirewallSet(r, current)
		if err != nil {
			return err
		}

		c.Recorder.Eventf(newSet, "Normal", "Create", "created firewallset %s", newSet.Name)

		ownedSets = append(ownedSets, newSet)

		return c.cleanupIntermediateSets(r, ownedSets)
	}

	err = c.syncFirewallSet(r, current)
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	if current.Status.ReadyReplicas != current.Spec.Replicas {
		r.Log.Info("set replicas are not yet ready, delaying old set cleanup")

		if time.Since(current.CreationTimestamp.Time) > c.progressDeadline {
			cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "ProgressDeadlineExceeded", fmt.Sprintf("FirewallSet %q has timed out progressing.", current.Name))
			r.Target.Status.Conditions.Set(cond)
		}

		return c.cleanupIntermediateSets(r, ownedSets)
	}

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetAvailable", fmt.Sprintf("FirewallSet %q has successfully progressed.", current.Name))
	r.Target.Status.Conditions.Set(cond)

	r.Log.Info("ensuring old sets are cleaned up")

	oldSets := controllers.Except(ownedSets, current)

	return c.deleteFirewallSets(r, oldSets...)
}

func (c *controller) cleanupIntermediateSets(r *controllers.Ctx[*v2.FirewallDeployment], sets []*v2.FirewallSet) error {
	// the idea is to keep the oldest and the latest set such that unfinished updates "in the middle" are cleaned up
	// prevents e.g. more than one firewall getting provisioned when triggering multiple spec changes quickly

	oldestSet, err := controllers.MinRevisionOf(sets)
	if err != nil {
		return err
	}

	latestSet, err := controllers.MaxRevisionOf(sets)
	if err != nil {
		return err
	}

	intermediateSets := controllers.Except(sets, oldestSet, latestSet)

	if len(intermediateSets) > 0 {
		r.Log.Info("cleaning up intermediate sets")

		err = c.deleteFirewallSets(r, intermediateSets...)
		if err != nil {
			return err
		}
	}

	return nil
}
