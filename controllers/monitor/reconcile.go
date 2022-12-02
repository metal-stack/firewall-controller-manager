package monitor

import (
	"fmt"
	"strconv"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	err := c.updateFirewallStatus(r)
	if err != nil {
		return err
	}

	return c.setRollAnnotation(r)
}

func (c *controller) updateFirewallStatus(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.SeedNamespace,
		},
	}
	err := c.Seed.Get(r.Ctx, client.ObjectKeyFromObject(fw), fw)
	if err != nil {
		return fmt.Errorf("associated firewall of monitor not found: %w", err)
	}

	if r.Target.ControllerStatus != nil {
		connection := &v2.ControllerConnection{
			ActualVersion: r.Target.ControllerStatus.ControllerVersion,
			Updated:       r.Target.ControllerStatus.Updated,
		}

		if connection.Updated.Time.IsZero() {
			cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet reconciled.")
			fw.Status.Conditions.Set(cond)
		} else if time.Since(connection.Updated.Time) > 5*time.Minute {
			cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s.", connection.Updated.Time.String()))
			fw.Status.Conditions.Set(cond)
		} else {
			cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", connection.Updated.Time.String()))
			fw.Status.Conditions.Set(cond)
		}
	} else {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionUnknown, "NotConnected", "Controller has not yet reconciled.")
		fw.Status.Conditions.Set(cond)
	}

	err = c.Seed.Status().Update(r.Ctx, fw)
	if err != nil {
		return fmt.Errorf("unable to update firewall status: %w", err)
	}

	return nil
}

func (c *controller) setRollAnnotation(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	v, ok := r.Target.Annotations[v2.RollSetAnnotation]
	if !ok {
		return nil
	}

	r.Log.Info("resource was annotated", "annotation", v2.RollSetAnnotation, "value", v)

	delete(r.Target.Annotations, v2.RollSetAnnotation)

	err := c.Shoot.Update(r.Ctx, r.Target)
	if err != nil {
		return err
	}

	r.Log.Info("cleaned up annotation")

	rollSet, err := strconv.ParseBool(v)
	if err != nil {
		r.Log.Error(err, "unable to parse annotation value, ignoring")
		return nil
	}

	if rollSet {
		r.Log.Info("initiating firewall set roll as requested by user annotation")

		fw := &v2.Firewall{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.Target.Name,
				Namespace: c.SeedNamespace,
			},
		}
		err = c.Seed.Get(r.Ctx, client.ObjectKeyFromObject(fw), fw)
		if err != nil {
			r.Log.Error(err, "unable to find out associated firewall in seed")
			return client.IgnoreNotFound(err)
		}

		ref := metav1.GetControllerOf(fw)
		if ref == nil {
			r.Log.Error(fmt.Errorf("no owner ref found"), "unable to find out associated firewall set in seed, aborting")
			return nil
		}

		set := &v2.FirewallSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: c.SeedNamespace,
			},
		}
		err = c.Seed.Get(r.Ctx, client.ObjectKeyFromObject(set), set)
		if err != nil {
			r.Log.Error(err, "unable to find out associated firewall set in seed")
			return client.IgnoreNotFound(err)
		}

		set.Annotations[v2.RollSetAnnotation] = strconv.FormatBool(true)

		err = c.Seed.Update(r.Ctx, set)
		if err != nil {
			return fmt.Errorf("unable to annotate firewall set: %w", err)
		}

		r.Log.Info("firewall set annotated")
	}

	return nil
}
