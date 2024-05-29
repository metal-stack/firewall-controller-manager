package firewall

import (
	"errors"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.Firewall], f *models.V1FirewallResponse) error {
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

func setMachineStatus(fw *v2.Firewall, f *models.V1FirewallResponse) error {
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

func getMachineStatus(f *models.V1FirewallResponse) (*v2.MachineStatus, error) {
	if f.ID == nil || f.Allocation == nil || f.Allocation.Created == nil || f.Liveliness == nil || f.Allocation.Image == nil {
		return nil, fmt.Errorf("firewall entity from metal-api is missing essential fields")
	}

	result := &v2.MachineStatus{
		MachineID:           *f.ID,
		AllocationTimestamp: metav1.NewTime(time.Time(*f.Allocation.Created)),
		Liveliness:          *f.Liveliness,
		ImageID:             pointer.SafeDeref(f.Allocation.Image.ID),
	}

	if f.Events != nil && f.Events.CrashLoop != nil {
		result.CrashLoop = *f.Events.CrashLoop
	}
	if f.Events != nil && len(f.Events.Log) > 0 && f.Events.Log[0].Event != nil {
		log := f.Events.Log[0]

		result.LastEvent = &v2.MachineLastEvent{
			Event:     *log.Event,
			Timestamp: metav1.NewTime(time.Time(log.Time)),
			Message:   log.Message,
		}
	}

	return result, nil
}

func (c *controller) setFirewallNetworks(r *controllers.Ctx[*v2.Firewall], f *models.V1FirewallResponse) error {
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
		n := n
		if n.Networkid == nil {
			continue
		}

		nw, err := c.networkCache.Get(r.Ctx, *n.Networkid)
		if err != nil {
			return err
		}

		result = append(result, v2.FirewallNetwork{
			ASN:                 n.Asn,
			DestinationPrefixes: n.Destinationprefixes,
			IPs:                 n.Ips,
			Nat:                 n.Nat,
			NetworkID:           n.Networkid,
			NetworkType:         n.Networktype,
			Prefixes:            nw.Prefixes,
			Vrf:                 n.Vrf,
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

	// Check if the firewall-controller has reconciled the shoot
	if connection.Updated.Time.IsZero() {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected to shoot.")
		fw.Status.Conditions.Set(cond)
	} else if time.Since(connection.Updated.Time) > 5*time.Minute {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s to shoot.", connection.Updated.Time.String()))
		fw.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled shoot at %s.", connection.Updated.Time.String()))
		fw.Status.Conditions.Set(cond)
	}

	// Check if the firewall-controller has reconciled the firewall
	if connection.SeedUpdated.Time.IsZero() {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected to seed.")
		fw.Status.Conditions.Set(cond)
	} else if time.Since(connection.SeedUpdated.Time) > 5*time.Minute {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s to seed.", connection.SeedUpdated.Time.String()))
		fw.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", connection.SeedUpdated.Time.String()))
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
}
