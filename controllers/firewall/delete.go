package firewall

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-go/api/client/machine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
	err := c.deleteFirewallMonitor(ctx, fw)
	if err != nil {
		return fmt.Errorf("unable to delete firewall monitor: %w", err)
	}

	fws, err := c.findAssociatedFirewalls(ctx, fw)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	if len(fws) == 0 {
		log.Info("firewall already deleted")
		return nil
	}

	if len(fws) > 1 {
		log.Error(fmt.Errorf("multiple associated firewalls found for deletion"), "deleting all of them", "amount", len(fws))
	}

	for _, f := range fws {
		f := f

		if f.ID == nil {
			continue
		}

		resp, err := c.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*f.ID).WithContext(ctx), nil)
		if err != nil {
			return fmt.Errorf("firewall delete error: %w", err)
		}

		log.Info("deleted firewall", "name", f.Name, "id", *resp.Payload.ID)

		c.Recorder.Eventf(fw, "Normal", "Delete", "deleted firewall %s id %s", fw.Name, *resp.Payload.ID)
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
