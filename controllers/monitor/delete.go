package monitor

import (
	"context"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, fw *v2.FirewallMonitor) error {
	return nil
}
