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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) setStatus(ctx context.Context, fw *v2.Firewall, m *models.V1FirewallResponse) error {
	var errors []error

	machineStatus, err := getMachineStatus(m)
	if err == nil {
		fw.Status.MachineStatus = machineStatus
	} else {
		errors = append(errors, err)
	}

	firewallNetworks, err := getFirewallNetworks(ctx, c.Metal, m)
	if err == nil {
		fw.Status.FirewallNetworks = firewallNetworks
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
			Asn:                 n.Asn,
			Destinationprefixes: n.Destinationprefixes,
			Ips:                 n.Ips,
			Nat:                 n.Nat,
			Networkid:           n.Networkid,
			Networktype:         n.Networktype,
			Prefixes:            nwResp.Payload.Prefixes,
			Vrf:                 n.Vrf,
		})
	}

	return result, nil
}
