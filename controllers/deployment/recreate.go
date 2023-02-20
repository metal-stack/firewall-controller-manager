package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

// recreateStrategy first deletes the existing firewall sets and then creates a new one
func (c *controller) recreateStrategy(r *controllers.Ctx[*v2.FirewallDeployment], ownedSets []*v2.FirewallSet, current *v2.FirewallSet) error {
	newSetRequired, err := c.isNewSetRequired(r, current)
	if err != nil {
		return err
	}

	if newSetRequired {
		r.Log.Info("significant changes detected in the spec, cleaning up old sets then create new firewall set")

		err = c.deleteFirewallSets(r, ownedSets...)
		if err != nil {
			return err
		}

		newSet, err := c.createNextFirewallSet(r, current)
		if err != nil {
			return err
		}

		c.recorder.Eventf(newSet, "Normal", "Recreate", "recreated firewallset old: %s new: %s", current.Name, newSet.Name)

		return nil
	}

	err = c.syncFirewallSet(r, current)
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	if current.Status.ReadyReplicas == current.Spec.Replicas {
		cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetAvailable", fmt.Sprintf("FirewallSet %q has successfully progressed.", current.Name))
		r.Target.Status.Conditions.Set(cond)
	}

	return nil
}
