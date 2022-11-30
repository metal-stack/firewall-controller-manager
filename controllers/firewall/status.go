package firewall

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-go/api/client/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) Status(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
	fws, err := c.findAssociatedFirewalls(ctx, fw)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	status := v2.FirewallStatus{
		MachineStatus: &v2.MachineStatus{},
	}

	if status.MachineStatus == nil {
		status.MachineStatus = &v2.MachineStatus{}
	}

	if len(fws) == 0 {
		status.MachineStatus.Message = "no firewall created"
		return nil
	}

	if len(fws) > 1 {
		status.MachineStatus.Message = "multiple associated firewalls found"
		return nil
	}

	current := fws[0]

	if current.Allocation == nil || current.Allocation.Created == nil || current.ID == nil || current.Liveliness == nil {
		return fmt.Errorf("created firewall is missing essential fields")
	}

	status.MachineStatus.MachineID = *current.ID
	status.MachineStatus.AllocationTimestamp = metav1.NewTime(time.Time(*current.Allocation.Created))
	status.MachineStatus.Liveliness = *current.Liveliness

	// check whether network prefixes were updated in metal-api
	// prefixes in the firewall machine allocation are just a snapshot when the firewall was created.
	// -> when changing prefixes in the referenced network the firewall does not know about any prefix changes.
	//
	// we replace the prefixes from the snapshot with the actual prefixes that are currently attached to the network.
	// this allows dynamic prefix reconfiguration of the firewall.
	status.FirewallNetworks = nil
	for _, n := range current.Allocation.Networks {
		n := n
		if n.Networkid == nil {
			continue
		}

		// TODO: network calls could be expensive, maybe add a cache for it
		nwResp, err := c.Metal.Network().FindNetwork(network.NewFindNetworkParams().WithID(*n.Networkid).WithContext(ctx), nil)
		if err != nil {
			return fmt.Errorf("network find error: %w", err)
		}

		fw.Status.FirewallNetworks = append(fw.Status.FirewallNetworks, v2.FirewallNetwork{
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

	if current.Events != nil && len(current.Events.Log) > 0 {
		log := current.Events.Log[0]

		if log.Event != nil {
			status.MachineStatus.Event = *log.Event
		}
		status.MachineStatus.Message = log.Message
		status.MachineStatus.EventTimestamp = metav1.NewTime(time.Time(log.Time))

		if current.Events.CrashLoop != nil {
			status.MachineStatus.CrashLoop = *current.Events.CrashLoop
		}
	}

	fw.Status = status

	return nil
}
