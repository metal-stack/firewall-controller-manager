package monitor

import (
	"fmt"
	"strconv"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(r *controllers.Ctx[*v2.FirewallMonitor]) error {
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

		fw := &v2.Firewall{}
		err = c.Seed.Get(r.Ctx, types.NamespacedName{Name: r.Target.Name, Namespace: c.SeedNamespace}, fw)
		if err != nil {
			r.Log.Error(err, "unable to find out associated firewall in seed")
			return client.IgnoreNotFound(err)
		}

		ref := metav1.GetControllerOf(fw)
		if ref == nil {
			r.Log.Error(fmt.Errorf("no owner ref found"), "unable to find out associated firewall set in seed, aborting")
			return nil
		}

		set := &v2.FirewallSet{}
		err = c.Seed.Get(r.Ctx, types.NamespacedName{Name: ref.Name, Namespace: c.SeedNamespace}, set)
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
