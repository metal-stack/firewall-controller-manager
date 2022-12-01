package firewall

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) ensureFirewallMonitor(r *controllers.Ctx[*v2.Firewall]) (*v2.FirewallMonitor, error) {
	var err error

	defer func() {
		if err != nil {
			cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionFalse, "Error", fmt.Sprintf("Monitor could not be deployed: %s", err))
			r.Target.Status.Conditions.Set(cond)
		}

		r.Log.Info("firewall monitor deployed")

		cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionTrue, "Deployed", "Successfully deployed firewall-monitor.")
		r.Target.Status.Conditions.Set(cond)
	}()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.ShootNamespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Shoot, ns, func() error {
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure namespace for monitor resource: %w", err)
	}

	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.ShootNamespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Shoot, mon, func() error {
		mon.Size = r.Target.Spec.Size
		mon.Image = r.Target.Spec.Image
		mon.Partition = r.Target.Spec.Partition
		mon.Project = r.Target.Spec.Project
		mon.Networks = r.Target.Spec.Networks
		mon.RateLimits = r.Target.Spec.RateLimits
		mon.EgressRules = r.Target.Spec.EgressRules
		mon.LogAcceptedConnections = r.Target.Spec.LogAcceptedConnections
		mon.MachineStatus = r.Target.Status.MachineStatus
		mon.Conditions = r.Target.Status.Conditions
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure firewall monitor resource: %w", err)
	}

	return mon, nil
}
