package monitor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	_, err := c.updateFirewallStatus(r)
	if err != nil {
		r.Log.Error(err, "unable to update firewall status")
		return controllers.RequeueAfter(3*time.Second, "unable to update firewall status, retrying")
	}

	err = c.rollSetAnnotation(r)
	if err != nil {
		r.Log.Error(err, "unable to handle roll set annotation")
		return err
	}

	return controllers.RequeueAfter(2*time.Minute, "continue reconciling monitor")
}

func (c *controller) updateFirewallStatus(r *controllers.Ctx[*v2.FirewallMonitor]) (*v2.Firewall, error) {
	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.c.GetSeedNamespace(),
		},
	}
	err := c.c.GetSeedClient().Get(r.Ctx, client.ObjectKeyFromObject(fw), fw)
	if err != nil {
		return nil, fmt.Errorf("associated firewall of monitor not found: %w", err)
	}

	firewall.SetFirewallStatusFromMonitor(fw, r.Target)

	err = c.c.GetSeedClient().Status().Update(r.Ctx, fw)
	if err != nil {
		return nil, fmt.Errorf("unable to update firewall status: %w", err)
	}

	return fw, nil
}

func (c *controller) rollSetAnnotation(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	rollSet := v2.IsAnnotationTrue(r.Target, v2.RollSetAnnotation)
	if !rollSet {
		return nil
	}

	err := v2.RemoveAnnotation(r.Ctx, c.c.GetShootClient(), r.Target, v2.RollSetAnnotation)
	if err != nil {
		return err
	}

	r.Log.Info("initiating firewall set roll as requested by user annotation")

	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.c.GetSeedNamespace(),
		},
	}

	set, err := findCorrespondingSet(r.Ctx, c.c.GetSeedClient(), fw)
	if err != nil {
		return client.IgnoreNotFound(err)
	}

	set.Annotations[v2.RollSetAnnotation] = strconv.FormatBool(true)

	err = c.c.GetSeedClient().Update(r.Ctx, set)
	if err != nil {
		return fmt.Errorf("unable to annotate firewall set: %w", err)
	}

	r.Log.Info("firewall set annotated")

	return nil
}

func findCorrespondingSet(ctx context.Context, c client.Client, fw *v2.Firewall) (*v2.FirewallSet, error) {
	err := c.Get(ctx, client.ObjectKeyFromObject(fw), fw)
	if err != nil {
		return nil, fmt.Errorf("unable to find out associated firewall in seed: %w", err)
	}

	ref := metav1.GetControllerOf(fw)
	if ref == nil {
		return nil, fmt.Errorf("unable to find out associated firewall set in seed: no owner ref found")
	}

	set := &v2.FirewallSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: fw.Namespace,
		},
	}
	err = c.Get(ctx, client.ObjectKeyFromObject(set), set)
	if err != nil {
		return nil, fmt.Errorf("unable to find out associated firewall set in seed: %w", err)
	}

	return set, nil
}
