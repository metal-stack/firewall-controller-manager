package set

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

func (c *controller) setStatus(ctx context.Context, set *v2.FirewallSet) error {
	ownedFirewalls, err := controllers.GetOwnedResources(ctx, c.Seed, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	set.Status.TargetReplicas = set.Spec.Replicas

	set.Status.ReadyReplicas = 0
	set.Status.ProgressingReplicas = 0
	set.Status.UnhealthyReplicas = 0

	for _, fw := range ownedFirewalls {
		var (
			fw = fw

			created = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallCreated)).Status == v2.ConditionTrue
			ready   = pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallReady)).Status == v2.ConditionTrue
			// FIXME: enable back in real environment:
			// connected = fw.Status.Conditions.Get(v2.FirewallControllerConnected).Status == v2.ConditionTrue
		)

		if created && ready {
			set.Status.ReadyReplicas++
			continue
		}

		if created && time.Since(pointer.SafeDeref(fw.Status.MachineStatus).AllocationTimestamp.Time) < c.FirewallHealthTimeout {
			set.Status.ProgressingReplicas++
			continue
		}

		set.Status.UnhealthyReplicas++
	}

	revision, err := controllers.Revision(set)
	if err != nil {
		return err
	}
	set.Status.ObservedRevision = revision

	return nil
}
