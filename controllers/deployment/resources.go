package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

func (c *controller) ensureFirewallControllerRBAC(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	r.Log.Info("ensuring firewall controller rbac")

	var err error
	defer func() {
		if err != nil {
			r.Log.Error(err, "unable to ensure firewall controller rbac")

			cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionFalse, "Error", fmt.Sprintf("RBAC resources could not be provisioned %s", err))
			r.Target.Status.Conditions.Set(cond)

			return
		}

		cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, "Provisioned", "RBAC provisioned successfully.")
		r.Target.Status.Conditions.Set(cond)
	}()

	err = helper.EnsureFirewallControllerRBAC(r.Ctx, c.c.GetSeedConfig(), r.Target, c.c.GetShootNamespace(), c.c.GetShootAccess())

	return nil
}
