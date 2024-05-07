package deployment

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

// recreateStrategy first deletes the existing firewall sets and then creates a new one
func (c *controller) recreateStrategy(r *controllers.Ctx[*v2.FirewallDeployment], ownedSets []*v2.FirewallSet, latestSet *v2.FirewallSet) error {
	if c.isNewSetRequired(r, latestSet) {
		r.Log.Info("significant changes detected in the spec, create new scaled down firewall set, then cleaning up old sets")

		set, err := c.createNextFirewallSet(r, latestSet, &setOverrides{
			replicas: pointer.Pointer(0),
		})
		if err != nil {
			return err
		}

		c.recorder.Eventf(set, "Normal", "Recreate", "recreated firewallset old: %s new: %s", latestSet.Name, set.Name)

		latestSet = set
	}

	err := c.deleteFirewallSets(r, controllers.Except(ownedSets, latestSet)...)
	if err != nil {
		return err
	}

	err = c.syncFirewallSet(r, latestSet)
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	if latestSet.Status.ReadyReplicas != latestSet.Spec.Replicas {
		r.Log.Info("set replicas are not yet ready")

		if time.Since(latestSet.CreationTimestamp.Time) > c.c.GetProgressDeadline() {
			cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "ProgressDeadlineExceeded", fmt.Sprintf("FirewallSet %q has timed out progressing.", latestSet.Name))
			r.Target.Status.Conditions.Set(cond)
		}

		return nil
	}

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetAvailable", fmt.Sprintf("FirewallSet %q has successfully progressed.", latestSet.Name))
	r.Target.Status.Conditions.Set(cond)

	return nil
}
