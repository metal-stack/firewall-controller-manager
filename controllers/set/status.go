package set

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) Status(ctx context.Context, log logr.Logger, set *v2.FirewallSet) error {
	ownedFirewalls, err := controllers.GetOwnedResources(ctx, c.Seed, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	status := v2.FirewallSetStatus{}

	for _, fw := range ownedFirewalls {
		fw := fw

		// TODO: this has to be revamped
		if fw.Status.MachineStatus.Event == "Phoned Home" && fw.Status.MachineStatus.Liveliness == "Alive" {
			// FIXME: enable back in real environment:
			// if fw.Status.ControllerStatus != nil && !fw.Status.ControllerStatus.Updated.IsZero() {
			if time.Since(fw.Status.MachineStatus.EventTimestamp.Time) < c.FirewallHealthTimeout {
				status.ReadyReplicas++
			} else if time.Since(fw.Status.MachineStatus.AllocationTimestamp.Time) < c.FirewallHealthTimeout {
				status.ProgressingReplicas++
			} else {
				status.UnhealthyReplicas++
			}
		} else if fw.Status.MachineStatus.CrashLoop {
			status.UnhealthyReplicas++
		} else {
			status.ProgressingReplicas++
		}
	}

	set.Status = status

	return nil
}
