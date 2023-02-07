package firewall

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) ensureFirewallMonitor(r *controllers.Ctx[*v2.Firewall]) (*v2.FirewallMonitor, error) {
	var err error

	defer func() {
		if err != nil {
			r.Log.Error(err, "error deploying firewall monitor")

			cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionFalse, "NotDeployed", fmt.Sprintf("Monitor could not be deployed: %s", err))
			r.Target.Status.Conditions.Set(cond)

			return
		}

		r.Log.Info("firewall monitor deployed")

		cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionTrue, "Deployed", "Successfully deployed firewall-monitor.")
		r.Target.Status.Conditions.Set(cond)

		return
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
		Size:                   r.Target.Spec.Size,
		Image:                  r.Target.Spec.Image,
		Partition:              r.Target.Spec.Partition,
		Project:                r.Target.Spec.Project,
		Networks:               r.Target.Spec.Networks,
		RateLimits:             r.Target.Spec.RateLimits,
		EgressRules:            r.Target.Spec.EgressRules,
		LogAcceptedConnections: r.Target.Spec.LogAcceptedConnections,
		MachineStatus:          r.Target.Status.MachineStatus,
		Conditions:             r.Target.Status.Conditions,
	}

	// on purpose not using controllerutil.CreateOrUpdate because it will not trigger an empty update
	// event in case nothing changes, such that the firewall monitor controller will not be started

	err = c.Shoot.Get(r.Ctx, client.ObjectKeyFromObject(mon), mon)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = c.Shoot.Create(r.Ctx, mon)
			if err != nil {
				return nil, fmt.Errorf("unable to create firewall monitor resource: %w", err)
			}

			return mon, nil
		}
		return nil, fmt.Errorf("unable to get firewall monitor resource: %w", err)
	}

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

	err = c.Shoot.Update(r.Ctx, mon)
	if err != nil {
		return nil, fmt.Errorf("unable to update firewall monitor resource: %w", err)
	}

	return mon, nil
}
