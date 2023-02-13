package monitor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallMonitor]) error {
	fw, err := c.updateFirewallStatus(r)
	if err != nil {
		r.Log.Error(err, "unable to update firewall status")
		return controllers.RequeueAfter(3*time.Second, "unable to update firewall status, retrying: %w")
	}

	err = c.offerFirewallControllerMigrationSecret(r, fw)
	if err != nil {
		r.Log.Error(err, "unable to offer firewall-controller migration secret")
		return controllers.RequeueAfter(10*time.Second, "unable to offer firewall-controller migration secret, retrying")
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
			Namespace: c.SeedNamespace,
		},
	}
	err := c.Seed.Get(r.Ctx, client.ObjectKeyFromObject(fw), fw)
	if err != nil {
		return nil, fmt.Errorf("associated firewall of monitor not found: %w", err)
	}

	if enabled, err := strconv.ParseBool(fw.Annotations[v2.FirewallNoControllerConnectionAnnotation]); err == nil && enabled {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "NotChecking", "Not checking controller connection due to firewall annotation.")
		fw.Status.Conditions.Set(cond)
	} else {
		if r.Target.ControllerStatus == nil {
			cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected.")
			fw.Status.Conditions.Set(cond)
		} else {
			connection := &v2.ControllerConnection{
				ActualVersion: r.Target.ControllerStatus.ControllerVersion,
				Updated:       r.Target.ControllerStatus.Updated,
			}

			if connection.Updated.Time.IsZero() {
				cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected.")
				fw.Status.Conditions.Set(cond)
			} else if time.Since(connection.Updated.Time) > 5*time.Minute {
				cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s.", connection.Updated.Time.String()))
				fw.Status.Conditions.Set(cond)
			} else {
				cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", connection.Updated.Time.String()))
				fw.Status.Conditions.Set(cond)
			}
		}
	}

	err = c.Seed.Status().Update(r.Ctx, fw)
	if err != nil {
		return nil, fmt.Errorf("unable to update firewall status: %w", err)
	}

	return fw, nil
}

// offerFirewallControllerMigrationSecret provides a secret that the firewall-controller can use to update from v1.x to v2.x
//
// this function can be removed when all firewall-controllers are running v2.x or newer.
func (c *controller) offerFirewallControllerMigrationSecret(r *controllers.Ctx[*v2.FirewallMonitor], fw *v2.Firewall) error {
	if metav1.GetControllerOf(fw) == nil {
		// it can be that there is no set or deployment governing the firewall.
		// in this case there may be no rbac resources deployed for seed access, so we cannot offer a migration secret.
		return nil
	}

	migrationSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v2.FirewallControllerMigrationSecretName,
			Namespace: c.Namespace,
		},
	}

	isOldController := pointer.SafeDeref(fw.Status.Conditions.Get(v2.FirewallControllerConnected)).Reason == "NotChecking" && r.Target.ControllerStatus == nil
	if !isOldController {
		// firewall-controller is already running with version v2.x or later, not offering migration secret
		return client.IgnoreNotFound(c.Shoot.Delete(r.Ctx, migrationSecret))
	}

	r.Log.Info("firewall-controller seems to be running with v1.x, offering migration secret")

	set, err := findCorrespondingSet(r.Ctx, c.Seed, fw)
	if err != nil {
		return err
	}

	ref := metav1.GetControllerOf(set)
	if ref == nil {
		return fmt.Errorf("unable to find out associated firewall deployment in seed: no owner ref found")
	}

	kubeconfig, err := helper.SeedAccessKubeconfig(&helper.SeedAccessConfig{
		Ctx:          r.Ctx,
		Client:       c.Seed,
		K8sVersion:   c.K8sVersion,
		Namespace:    c.SeedNamespace,
		ApiServerURL: c.APIServerURL,
		Deployment: &v2.FirewallDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ref.Name,
				Namespace: c.SeedNamespace,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("error creating kubeconfig for firewall-controller migration secret: %w", err)
	}

	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Shoot, migrationSecret, func() error {
		migrationSecret.Data = map[string][]byte{
			"kubeconfig": kubeconfig,
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring firewall-controller migration secret: %w", err)
	}

	return nil
}

func (c *controller) rollSetAnnotation(r *controllers.Ctx[*v2.FirewallMonitor]) error {
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

		set, err := findCorrespondingSet(r.Ctx, c.Seed, fw)
		if err != nil {
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
