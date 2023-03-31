package firewall

import (
	"context"
	"errors"
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) setStatus(r *controllers.Ctx[*v2.Firewall], m *models.V1FirewallResponse) error {
	var errs []error

	machineStatus, err := getMachineStatus(m)
	if err == nil {
		r.Target.Status.MachineStatus = machineStatus
	} else {
		errs = append(errs, err)
	}

	firewallNetworks, err := getFirewallNetworks(r.Ctx, c.networkCache, m)
	if err == nil {
		r.Target.Status.FirewallNetworks = firewallNetworks
	} else {
		errs = append(errs, err)
	}

	if v2.IsAnnotationTrue(r.Target, v2.FirewallNoControllerConnectionAnnotation) {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "NotChecking", "Not checking controller connection due to firewall annotation.")
		r.Target.Status.Conditions.Set(cond)
	} else if r.Target.Status.ControllerStatus == nil {
		cond := v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "NotConnected", "Controller has not yet connected.")
		r.Target.Status.Conditions.Set(cond)
	}

	r.Target.Status.ShootAccess = c.c.GetShootAccess()

	return errors.Join(errs...)
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

func getFirewallNetworks(ctx context.Context, cache *cache.Cache[string, *models.V1NetworkResponse], m *models.V1FirewallResponse) ([]v2.FirewallNetwork, error) {
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

		nw, err := cache.Get(ctx, *n.Networkid)
		if err != nil {
			return nil, err
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

	return result, nil
}
