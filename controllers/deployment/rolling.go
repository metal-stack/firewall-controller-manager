package deployment

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

// rollingUpdateStrategy first creates a new set and deletes the old one's when the new one becomes ready
func (c *controller) rollingUpdateStrategy(r *controllers.Ctx[*v2.FirewallDeployment], ownedSets []*v2.FirewallSet, latestSet *v2.FirewallSet) error {
	if c.isNewSetRequired(r, latestSet) {
		r.Log.Info("significant changes detected in the spec, creating new firewall set", "distance", v2.FirewallRollingUpdateSetDistance)

		newSet, err := c.createNextFirewallSet(r, latestSet, &setOverrides{
			distance: v2.FirewallRollingUpdateSetDistance.Pointer(),
		})
		if err != nil {
			return err
		}

		c.recorder.Eventf(newSet, nil, "Normal", "Create", "created firewallset %s", newSet.Name)

		ownedSets = append(ownedSets, newSet)

		return c.cleanupIntermediateSets(r, ownedSets)
	}

	err := c.syncFirewallSet(r, latestSet)
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	if latestSet.Status.ReadyReplicas != latestSet.Spec.Replicas {
		r.Log.Info("set replicas are not yet ready")

		if time.Since(latestSet.CreationTimestamp.Time) > c.c.GetProgressDeadline() {
			cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "ProgressDeadlineExceeded", fmt.Sprintf("FirewallSet %q has timed out progressing.", latestSet.Name))
			r.Target.Status.Conditions.Set(cond)
		}

		return c.cleanupIntermediateSets(r, ownedSets)
	}

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetAvailable", fmt.Sprintf("FirewallSet %q has successfully progressed.", latestSet.Name))
	r.Target.Status.Conditions.Set(cond)

	r.Log.Info("ensuring old sets are cleaned up")

	oldSets := controllers.Except(ownedSets, latestSet)

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
