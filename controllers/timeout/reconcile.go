package timeout

import (
	"context"
	"fmt"
	"sort"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

func (c *controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != c.namespace { // should already be filtered out through predicate, but we will check anyway
		return ctrl.Result{}, nil
	}

	set := &v2.FirewallSet{}
	if err := c.client.Get(ctx, req.NamespacedName, set, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			c.log.Info("resource no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("error retrieving resource: %w", err)
	}

	if !set.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	ownedFirewalls, _, err := controllers.GetOwnedResources(ctx, c.c.GetSeedClient(), set.Spec.Selector, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	return c.deleteIfUnhealthyOrTimeout(ctx, ownedFirewalls...)
}

func (c *controller) deleteIfUnhealthyOrTimeout(ctx context.Context, fws ...*v2.Firewall) (ctrl.Result, error) {
	type fwWithStatus struct {
		firewall *v2.Firewall
		status   *v2.FirewallStatusEvalResult
	}

	var nextTimeouts []*fwWithStatus

	for _, fw := range fws {
		status := v2.EvaluateFirewallStatus(fw, c.c.GetCreateTimeout(), c.c.GetFirewallHealthTimeout())

		switch status.Result {
		case v2.FirewallStatusCreateTimeout, v2.FirewallStatusHealthTimeout:
			c.log.Info("firewall timeout exceeded, deleting from set", "reason", status.Reason, "firewall-name", fw.Name)

			if fw.DeletionTimestamp != nil {
				c.log.Info("deletion timestamp on firewall already set", "firewall-name", fw.Name)
				continue
			}

			err := c.c.GetSeedClient().Delete(ctx, fw)
			if err != nil {
				return ctrl.Result{}, err
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

		var (
			nextTimeout = nextTimeouts[0]
			in          = *nextTimeout.status.TimeoutIn
		)

		c.log.Info("scheduled check for next health timeout", "firewall-name", nextTimeout.firewall, "reason", nextTimeout.status.Reason, "in", in.String())

		return ctrl.Result{
			RequeueAfter: in,
		}, nil
	}

	return ctrl.Result{}, nil
}
