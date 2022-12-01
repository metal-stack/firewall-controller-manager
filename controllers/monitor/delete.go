package monitor

import (
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) Delete(_ *controllers.Ctx[*v2.FirewallMonitor]) error {
	return nil
}
