package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) recreateStrategy(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment) error {
	ownedSets, err := controllers.GetOwnedResources(ctx, c.Seed, deploy, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	newestSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if newestSet == nil {
		log.Info("no firewall set is present, creating a new one")

		set, err := c.createFirewallSet(ctx, log, deploy, 0)
		if err != nil {
			cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "FirewallSetCreateError", fmt.Sprintf("Error creating firewall set: %s", err))
			deploy.Status.Conditions.Set(cond)

			return fmt.Errorf("unable to create firewall set: %w", err)
		}

		cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetCreated", fmt.Sprintf("Created new firewall set %q", set.Name))
		deploy.Status.Conditions.Set(cond)

		c.Recorder.Eventf(set, "Normal", "Create", "created firewallset %s", set.Name)

		return nil
	}

	newSetRequired, err := c.isNewSetRequired(ctx, log, deploy, newestSet)
	if err != nil {
		return err
	}

	if newSetRequired {
		log.Info("significant changes detected in the spec, cleaning up old sets then create new firewall set")

		err = c.deleteFirewallSets(ctx, log, ownedSets)
		if err != nil {
			return err
		}

		revision, err := controllers.NextRevision(newestSet)
		if err != nil {
			return err
		}

		newSet, err := c.createFirewallSet(ctx, log, deploy, revision)
		if err != nil {
			cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionFalse, "FirewallSetCreateError", fmt.Sprintf("Error creating firewall set: %s", err))
			deploy.Status.Conditions.Set(cond)

			return err
		}

		log.Info("created new firewall set", "name", newSet.Name)

		cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetCreated", fmt.Sprintf("Created new firewall set %q", newSet.Name))
		deploy.Status.Conditions.Set(cond)

		c.Recorder.Eventf(newSet, "Normal", "Recreate", "recreated firewallset old: %s new: %s", newestSet.Name, newSet.Name)

		return nil
	}

	log.Info("existing firewall set does not need to be rolled, only updating the resource")

	newestSet.Spec.Replicas = deploy.Spec.Replicas
	newestSet.Spec.Template = deploy.Spec.Template

	err = c.Seed.Update(ctx, newestSet, &client.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	log.Info("updated firewall set", "name", newestSet.Name)

	cond := v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "FirewallSetUpdated", fmt.Sprintf("Updated firewall set %q", newestSet.Name))
	deploy.Status.Conditions.Set(cond)

	c.Recorder.Eventf(newestSet, "Normal", "Update", "updated firewallset %s", newestSet.Name)

	if newestSet.Status.ReadyReplicas == newestSet.Spec.Replicas {
		cond = v2.NewCondition(v2.FirewallDeplomentProgressing, v2.ConditionTrue, "NewFirewallSetAvailable", fmt.Sprintf("FirewallSet %q has successfully progressed.", newestSet.Name))
		deploy.Status.Conditions.Set(cond)
	}

	log.Info("ensuring old sets are cleaned up")

	oldSets := controllers.Except(ownedSets, newestSet)

	return c.deleteFirewallSets(ctx, log, oldSets)
}
