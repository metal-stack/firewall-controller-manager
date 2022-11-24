package controllers

import (
	"fmt"
)

func FirewallSetTag(setName string) string {
	return fmt.Sprintf("metal.stack.io/firewall-controller-manager/set=%s", setName)
}
