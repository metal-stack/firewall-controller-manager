package timeout

import (
	"fmt"
	"sort"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"

	corev1 "k8s.io/api/core/v1"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallSet]) error {
	ownedFirewalls, _, err := controllers.GetOwnedResources(r.Ctx, c.c.GetSeedClient(), r.Target.Spec.Selector, r.Target, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	err = c.deleteIfUnhealthyOrTimeout(r, ownedFirewalls...)
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) deleteIfUnhealthyOrTimeout(r *controllers.Ctx[*v2.FirewallSet], fws ...*v2.Firewall) error {
	type fwWithStatus struct {
		firewall *v2.Firewall
		status   *v2.FirewallStatusEvalResult
	}

	var nextTimeouts []*fwWithStatus

	for _, fw := range fws {
		status := v2.EvaluateFirewallStatus(fw, c.c.GetCreateTimeout(), c.c.GetFirewallHealthTimeout())

		switch status.Result {
		case v2.FirewallStatusCreateTimeout, v2.FirewallStatusHealthTimeout:
			r.Log.Info("firewall timeout exceeded, deleting from set", "reason", status.Reason, "firewall-name", fw.Name)

			if fw.DeletionTimestamp != nil {
				r.Log.Info("deletion timestamp on firewall already set", "firewall-name", fw.Name)
				continue
			}

			err := c.c.GetSeedClient().Delete(r.Ctx, fw)
			if err != nil {
				return err
			}

			c.recorder.Eventf(fw, nil, corev1.EventTypeNormal, "Delete", "deleting firewall", "deleted firewall %s due to %s", fw.Name, status)

		case v2.FirewallStatusUnhealthy:
			if status.TimeoutIn != nil {
				nextTimeouts = append(nextTimeouts, &fwWithStatus{
					firewall: fw,
					status:   status,
				})
			}
		}
	}

	if len(nextTimeouts) > 0 {
		sort.SliceStable(nextTimeouts, func(i, j int) bool {
			return *nextTimeouts[i].status.TimeoutIn < *nextTimeouts[j].status.TimeoutIn
		})

		nextTimeout := nextTimeouts[0]

		return controllers.RequeueAfter(
			*nextTimeout.status.TimeoutIn,
			fmt.Sprintf("checking for timeout of firewall %q (reason: %s)", nextTimeout.firewall.Name, nextTimeout.status.Reason))
	}

	return nil
}
