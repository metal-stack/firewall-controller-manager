package firewall

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-go/api/client/machine"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
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
