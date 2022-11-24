package controllers

import (
	"fmt"
)

func FirewallDeploymentTag(firewallDeploymentName string) string {
	return fmt.Sprintf("metal.stack.io/firewall-controller-manager/deployment=%s", firewallDeploymentName)
}
