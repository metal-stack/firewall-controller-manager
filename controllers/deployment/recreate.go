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

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if lastSet == nil {
		log.Info("no firewall set is present, creating a new one")

		set, err := c.createFirewallSet(ctx, log, deploy, 0)
		if err != nil {
			return fmt.Errorf("unable to create firewall set: %w", err)
		}

		c.Recorder.Eventf(set, "Normal", "Create", "created firewallset %s", set.Name)

		return nil
	}

	newSetRequired, err := c.isNewSetRequired(ctx, log, deploy, lastSet)
	if err != nil {
		return err
	}

	if newSetRequired {
		log.Info("significant changes detected in the spec, cleaning up old sets then create new firewall set")

		err = c.deleteFirewallSets(ctx, log, ownedSets)
		if err != nil {
			return err
		}

		revision, err := controllers.NextRevision(lastSet)
		if err != nil {
			return err
		}

		newSet, err := c.createFirewallSet(ctx, log, deploy, revision)
		if err != nil {
			return err
		}

		log.Info("created new firewall set", "name", newSet.Name)

		c.Recorder.Eventf(newSet, "Normal", "Recreate", "recreated firewallset old: %s new: %s", lastSet.Name, newSet.Name)

		return nil
	}

	log.Info("existing firewall set does not need to be rolled, only updating the resource")

	lastSet.Spec.Replicas = deploy.Spec.Replicas
	lastSet.Spec.Template = deploy.Spec.Template

	err = c.Seed.Update(ctx, lastSet, &client.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("unable to update firewall set: %w", err)
	}

	log.Info("updated firewall set", "name", lastSet.Name)

	c.Recorder.Eventf(lastSet, "Normal", "Update", "updated firewallset %s", lastSet.Name)

	log.Info("ensuring old sets are cleaned up")

	oldSets := controllers.Except(ownedSets, lastSet)

	return c.deleteFirewallSets(ctx, log, oldSets)
}
