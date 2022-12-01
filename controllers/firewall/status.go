package firewall

import (
	"context"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.Firewall], m *models.V1FirewallResponse) error {
	var errors []error

	machineStatus, err := getMachineStatus(m)
	if err == nil {
		r.Target.Status.MachineStatus = machineStatus
	} else {
		errors = append(errors, err)
	}

	firewallNetworks, err := getFirewallNetworks(r.Ctx, c.Metal, m)
	if err == nil {
		r.Target.Status.FirewallNetworks = firewallNetworks
	} else {
		errors = append(errors, err)
	}

	controllerConnection, err := c.getControllerConnectionStatus(r)
	if err == nil {
		r.Target.Status.ControllerStatus = controllerConnection
	} else {
		errors = append(errors, err)
	}

	return controllers.CombineErrors(errors...)
}

func getMachineStatus(m *models.V1FirewallResponse) (*v2.MachineStatus, error) {
	result := &v2.MachineStatus{}

	if m.ID == nil || m.Allocation == nil || m.Allocation.Created == nil || m.Liveliness == nil {
		return nil, fmt.Errorf("firewall entity is missing essential fields")
	}

	result.MachineID = *m.ID
	result.AllocationTimestamp = metav1.NewTime(time.Time(*m.Allocation.Created))
	result.Liveliness = *m.Liveliness

	if m.Events != nil && m.Events.CrashLoop != nil {
		result.CrashLoop = *m.Events.CrashLoop
	}
	if m.Events != nil && len(m.Events.Log) > 0 && m.Events.Log[0].Event != nil {
		log := m.Events.Log[0]

		result.LastEvent = &v2.MachineLastEvent{
			Event:     *log.Event,
			Timestamp: metav1.NewTime(time.Time(log.Time)),
			Message:   log.Message,
		}
	}

	return result, nil
}

func getFirewallNetworks(ctx context.Context, client metalgo.Client, m *models.V1FirewallResponse) ([]v2.FirewallNetwork, error) {
	// check whether network prefixes were updated in metal-api
	// prefixes in the firewall machine allocation are just a snapshot when the firewall was created.
	// -> when changing prefixes in the referenced network the firewall does not know about any prefix changes.
	//
	// we replace the prefixes from the snapshot with the actual prefixes that are currently attached to the network.
	// this allows dynamic prefix reconfiguration of the firewall.

	if m.Allocation == nil {
		return nil, fmt.Errorf("firewall entity is missing essential fields")
	}

	var result []v2.FirewallNetwork
	for _, n := range m.Allocation.Networks {
		n := n
		if n.Networkid == nil {
			continue
		}

		// TODO: network calls could be expensive, maybe add a cache for it
		nwResp, err := client.Network().FindNetwork(network.NewFindNetworkParams().WithID(*n.Networkid).WithContext(ctx), nil)
		if err != nil {
			return nil, fmt.Errorf("network find error: %w", err)
		}

		result = append(result, v2.FirewallNetwork{
			ASN:                 n.Asn,
			DestinationPrefixes: n.Destinationprefixes,
			IPs:                 n.Ips,
			Nat:                 n.Nat,
			NetworkID:           n.Networkid,
			NetworkType:         n.Networktype,
			Prefixes:            nwResp.Payload.Prefixes,
			Vrf:                 n.Vrf,
		})
	}

	return result, nil
}

func (c *controller) getControllerConnectionStatus(r *controllers.Ctx[*v2.Firewall]) (*v2.ControllerConnection, error) {
	monProvisioned := pointer.SafeDeref(r.Target.Status.Conditions.Get(v2.FirewallMonitorDeployed)).Status == v2.ConditionTrue
	if !monProvisioned {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionUnknown, "MonitorNotProvisioned", "Monitor was not yet deployed into the shoot.")
		r.Target.Status.Conditions.Set(cond)

		return nil, nil
	}

	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.Target.Name,
			Namespace: c.ShootNamespace,
		},
	}
	err := c.Shoot.Get(r.Ctx, client.ObjectKeyFromObject(mon), mon)
	if err != nil {
		if apierrors.IsNotFound(err) {
			cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionUnknown, "MonitorNotFound", fmt.Sprintf("Monitor is not present in shoot: %s/%s.", mon.Namespace, mon.Name))
			r.Target.Status.Conditions.Set(cond)

			return nil, nil
		}

		return nil, err
	}

	connection := &v2.ControllerConnection{
		ActualVersion: pointer.SafeDeref(mon.ControllerStatus).ControllerVersion,
		Updated:       pointer.SafeDeref(mon.ControllerStatus).Updated,
	}

	if connection.Updated.Time.IsZero() {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet reconciled.")
		r.Target.Status.Conditions.Set(cond)
	} else if time.Since(connection.Updated.Time) > 5*time.Minute {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", fmt.Sprintf("Controller has stopped reconciling since %s.", connection.Updated.Time.String()))
		r.Target.Status.Conditions.Set(cond)
	} else {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", connection.Updated.Time.String()))
		r.Target.Status.Conditions.Set(cond)
	}

	return connection, nil
}
