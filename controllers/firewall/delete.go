package firewall

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/machine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(r *controllers.Ctx[*v2.Firewall]) error {
	err := c.deleteFirewallMonitor(r.Ctx, r.Target)
	if err != nil {
		return fmt.Errorf("unable to delete firewall monitor: %w", err)
	}

	fws, err := c.findAssociatedFirewalls(r.Ctx, r.Target)
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
		f := f

		if f.ID == nil {
			continue
		}

		resp, err := c.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*f.ID).WithContext(r.Ctx), nil)
		if err != nil {
			return fmt.Errorf("firewall delete error: %w", err)
		}

		r.Log.Info("deleted firewall", "name", f.Name, "id", *resp.Payload.ID)

		c.Recorder.Eventf(r.Target, "Normal", "Delete", "deleted firewall %s id %s", r.Target.Name, *resp.Payload.ID)
	}

	return nil
}

func (c *controller) deleteFirewallMonitor(ctx context.Context, fw *v2.Firewall) error {
	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fw.Name,
			Namespace: c.ShootNamespace,
		},
	}

	err := c.Shoot.Delete(ctx, mon, &client.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}
