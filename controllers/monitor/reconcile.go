package monitor

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, mon *v2.FirewallMonitor) error {
	v, ok := mon.Annotations[v2.RollSetAnnotation]
	if !ok {
		return nil
	}

	log.Info("resource was annotated", "annotation", v2.RollSetAnnotation, "value", v)

	delete(mon.Annotations, v2.RollSetAnnotation)

	err := c.Shoot.Update(ctx, mon)
	if err != nil {
		return err
	}

	log.Info("cleaned up annotation")

	rollSet, err := strconv.ParseBool(v)
	if err != nil {
		log.Error(err, "unable to parse annotation value, ignoring")
		return nil
	}

	if rollSet {
		log.Info("initiating firewall set roll as requested by user annotation")

		fw := &v2.Firewall{}
		err = c.Seed.Get(ctx, types.NamespacedName{Name: mon.Name, Namespace: c.SeedNamespace}, fw)
		if err != nil {
			log.Error(err, "unable to find out associated firewall in seed")
			return client.IgnoreNotFound(err)
		}

		ref := metav1.GetControllerOf(fw)
		if ref == nil {
			log.Error(fmt.Errorf("no owner ref found"), "unable to find out associated firewall set in seed, aborting")
			return nil
		}

		set := &v2.FirewallSet{}
		err = c.Seed.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: c.SeedNamespace}, set)
		if err != nil {
			log.Error(err, "unable to find out associated firewall set in seed")
			return client.IgnoreNotFound(err)
		}

		set.Annotations[v2.RollSetAnnotation] = strconv.FormatBool(true)

		err = c.Seed.Update(ctx, set)
		if err != nil {
			return fmt.Errorf("unable to annotate firewall set: %w", err)
		}

		log.Info("firewall set annotated")
	}

	return nil
}
