package firewall

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/machine"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.Firewall]) error {
	err := c.deleteFirewallMonitor(r.Ctx, r.Target)
	if err != nil {
		return fmt.Errorf("unable to delete firewall monitor: %w", err)
	}

	fws, err := c.firewallCache.Get(r.Ctx, r.Target)
	if err != nil {
		return controllers.RequeueAfter(10*time.Second, err.Error())
	}

	if len(fws) == 0 {
		r.Log.Info("firewall already deleted")
		return nil
	}

	if len(fws) > 1 {
		r.Log.Error(fmt.Errorf("multiple associated firewalls found for deletion"), "deleting all of them", "amount", len(fws))
	}

	for _, f := range fws {
		if f.ID == nil {
			continue
		}

		resp, err := c.c.GetMetal().Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*f.ID).WithContext(r.Ctx), nil)
		if err != nil {
			r.Log.Error(err, "firewall deletion failed")

			return controllers.RequeueAfter(5*time.Second, "firewall deletion failed, retrying")
		}

		r.Log.Info("deleted firewall", "firewall-name", f.Name, "id", *resp.Payload.ID)

		c.recorder.Eventf(r.Target, nil, corev1.EventTypeNormal, "Delete", "deleting firewall", "deleted firewall %s id %s", r.Target.Name, *resp.Payload.ID)
	}

	return nil
}

func (c *controller) deleteFirewallMonitor(ctx context.Context, fw *v2.Firewall) error {
	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fw.Name,
			Namespace: c.c.GetShootNamespace(),
		},
	}

	err := c.c.GetShootClient().Delete(ctx, mon)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return controllers.RequeueAfter(1*time.Second, "waiting for firewall monitor to be deleted, requeuing")
}
