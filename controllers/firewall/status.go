package firewall

import (
	"errors"
	"fmt"
	"time"

	"github.com/metal-stack/api/go/enum"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.Firewall], f *apiv2.Machine) error {
	var errs []error

	err := setMachineStatus(r.Target, f)
	if err != nil {
		errs = append(errs, err)
	}

	err = c.setFirewallNetworks(r, f)
	if err != nil {
		errs = append(errs, err)
	}

	r.Target.Status.ShootAccess = c.c.GetShootAccess()

	return errors.Join(errs...)
}

func setMachineStatus(fw *v2.Firewall, f *apiv2.Machine) error {
	if f == nil {
		return nil
	}

	result, err := getMachineStatus(f)
	if err != nil {
		return err
	}

	fw.Status.MachineStatus = result

	return nil
}

func getMachineStatus(f *apiv2.Machine) (*v2.MachineStatus, error) {
	if f.Allocation == nil || f.Allocation.Meta == nil || f.Allocation.Meta.CreatedAt == nil || f.Allocation.Image == nil || f.Status == nil {
		return nil, fmt.Errorf("firewall entity from metal-api is missing essential fields")
	}

	liveliness, err := enum.GetStringValue(f.Status.Liveliness)
	if err != nil {
		return nil, err
	}

	result := &v2.MachineStatus{
		MachineID:           f.Uuid,
		AllocationTimestamp: metav1.NewTime(f.Allocation.Meta.CreatedAt.AsTime()),
		Liveliness:          *liveliness,
		ImageID:             f.Allocation.Image.Id,
	}

	if f.RecentProvisioningEvents != nil && f.RecentProvisioningEvents.State == apiv2.MachineProvisioningEventState_MACHINE_PROVISIONING_EVENT_STATE_CRASHLOOP {
		result.CrashLoop = true
	}

	if f.RecentProvisioningEvents != nil && len(f.RecentProvisioningEvents.Events) > 0 {
		event := f.RecentProvisioningEvents.Events[0]
		eventType, err := enum.GetStringValue(event.Event)
		if err != nil {
			return nil, err
		}
		result.LastEvent = &v2.MachineLastEvent{
			Event:     *eventType,
			Timestamp: metav1.NewTime(event.Time.AsTime()),
			Message:   event.Message,
		}
	}

	return result, nil
}

func (c *controller) setFirewallNetworks(r *controllers.Ctx[*v2.Firewall], f *apiv2.Machine) error {
	// check whether network prefixes were updated in metal-api
	// prefixes in the firewall machine allocation are just a snapshot when the firewall was created.
	// -> when changing prefixes in the referenced network the firewall does not know about any prefix changes.
	//
	// we replace the prefixes from the snapshot with the actual prefixes that are currently attached to the network.
	// this allows dynamic prefix reconfiguration of the firewall.

	if f == nil {
		return nil
	}

	if f.Allocation == nil {
		return fmt.Errorf("firewall entity is missing essential fields")
	}

	var result []v2.FirewallNetwork

	for _, n := range f.Allocation.Networks {
		nw, err := c.networkCache.Get(r.Ctx, n.Network)
		if err != nil {
			return err
		}
		networkType, err := enum.GetStringValue(n.NetworkType)
		if err != nil {
			return err
		}
		var nat bool
		if n.NatType == apiv2.NATType_NAT_TYPE_IPV4_MASQUERADE {
			nat = true
		}

		result = append(result, v2.FirewallNetwork{
			ASN:                 new(int64(n.Asn)),
			DestinationPrefixes: n.DestinationPrefixes,
			IPs:                 n.Ips,
			Nat:                 &nat,
			NetworkID:           &n.Network,
			NetworkType:         networkType,
			Prefixes:            nw.Prefixes,
			Vrf:                 new(int64(n.Vrf)),
		})
	}

	r.Target.Status.FirewallNetworks = result

	return nil
}

func SetFirewallStatusFromMonitor(fw *v2.Firewall, mon *v2.FirewallMonitor) {
	if v2.IsAnnotationTrue(fw, v2.FirewallNoControllerConnectionAnnotation) {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "NotChecking", "Not checking controller connection due to firewall annotation.")
		fw.Status.Conditions.Set(cond)

		cond = v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionTrue, "NotChecking", "Not checking controller seed connection due to firewall annotation.")
		fw.Status.Conditions.Set(cond)

		cond = v2.NewCondition(v2.FirewallDistanceConfigured, v2.ConditionTrue, "NotChecking", "Not checking distance due to firewall annotation.")
		fw.Status.Conditions.Set(cond)

		if isProvisioned(fw) {
			cond := v2.NewCondition(v2.FirewallProvisioned, v2.ConditionTrue, "Provisioned", "All firewall conditions have been met.")
			fw.Status.Conditions.Set(cond)
		}

		return
	}

	if mon == nil {
		return
	}

	if mon.ControllerStatus == nil {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected.")
		fw.Status.Conditions.Set(cond)

		cond = v2.NewCondition(v2.FirewallDistanceConfigured, v2.ConditionFalse, "NotConnected", "Controller has not yet connected.")
		fw.Status.Conditions.Set(cond)

		return
	}

	connection := &v2.ControllerConnection{
		ActualVersion:  mon.ControllerStatus.ControllerVersion,
		Updated:        mon.ControllerStatus.Updated,
		SeedUpdated:    mon.ControllerStatus.SeedUpdated,
		ActualDistance: mon.ControllerStatus.Distance,
	}

	fw.Status.ControllerStatus = connection

	var (
		// currently, the firewall-controller writes the reconcile time hard-coded every three minutes
		// the FCM reconciles the firewall hard-coded at least every two minutes
		//
		// this can be visualized as:
		//
		// fc (write)  w        w        w        w
		//             |        |        |        |
		// t (minutes) 0--1--2--3--4--5--6--7--8--9--10--
		//             |     |     |     |     |     |
		// FCM (read)  r     r     r     r     r     r
		//
		// so, read out data will contain t={0, 0, 3, 6, 6, 9}, which shows that the maximum distance is three minutes
		maximumSeedUpdateDrift = 3 * time.Minute
		// in this case, the firewall-controller almost permanently updates this value (fw.Spec.Interval, by default 10s)
		// so we can assume the read out interval from the fcm firewall reconcile, which is maximum two minutes as described above
		maximumShootUpdateDrift = 2 * time.Minute
	)

	// Check if the firewall-controller has reconciled the shoot
	if connection.Updated.Time.IsZero() {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected to shoot.")
		fw.Status.Conditions.Set(cond)
	} else if time.Since(connection.Updated.Time) > maximumShootUpdateDrift {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s to shoot.", connection.Updated.String()))
		fw.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled shoot at %s.", connection.Updated.String()))
		fw.Status.Conditions.Set(cond)
	}

	// Check if the firewall-controller has reconciled the firewall
	if connection.SeedUpdated.Time.IsZero() {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected to seed.")
		fw.Status.Conditions.Set(cond)
	} else if time.Since(connection.SeedUpdated.Time) > maximumSeedUpdateDrift {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s to seed.", connection.SeedUpdated.String()))
		fw.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", connection.SeedUpdated.String()))
		fw.Status.Conditions.Set(cond)
	}

	// Check if the firewall-controller has reconciled the distance
	if !mon.ControllerStatus.DistanceSupported {
		cond := v2.NewCondition(v2.FirewallDistanceConfigured, v2.ConditionTrue, "NotChecking", "Controller does not support distance reconciliation.")
		fw.Status.Conditions.Set(cond)
	} else if fw.Distance == connection.ActualDistance {
		cond := v2.NewCondition(v2.FirewallDistanceConfigured, v2.ConditionTrue, "Configured", fmt.Sprintf("Controller has configured the specified distance %d.", fw.Distance))
		fw.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallDistanceConfigured, v2.ConditionFalse, "NotConfigured", fmt.Sprintf("Controller has configured distance %d, but %d is specified.", connection.ActualDistance, fw.Distance))
		fw.Status.Conditions.Set(cond)
	}

	if isProvisioned(fw) {
		cond := v2.NewCondition(v2.FirewallProvisioned, v2.ConditionTrue, "Provisioned", "All firewall conditions have been met.")
		fw.Status.Conditions.Set(cond)
	}
}

func isProvisioned(fw *v2.Firewall) bool {
	for _, ct := range []v2.ConditionType{
		v2.FirewallCreated,
		v2.FirewallReady,
		v2.FirewallControllerConnected,
		v2.FirewallControllerSeedConnected,
		v2.FirewallDistanceConfigured,
	} {
		cond := fw.Status.Conditions.Get(ct)
		if cond == nil || cond.Status != v2.ConditionTrue {
			return false
		}
	}
	return true
}
