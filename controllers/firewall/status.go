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

	status := v2.FirewallStatus{}

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

// func reconcileEgressIPs(ctx context.Context, r *egressIPReconciler) error {
// 	currentEgressIPs := sets.NewString()

// 	resp, err := r.mclient.IP().FindIPs(ip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
// 		Projectid: r.infrastructureConfig.ProjectID,
// 		Tags:      []string{r.egressTag},
// 		Type:      models.V1IPBaseTypeStatic,
// 	}).WithContext(ctx), nil)
// 	if err != nil {
// 		return &reconciler.RequeueAfterError{
// 			Cause:        fmt.Errorf("failed to list egress ips of cluster %w", err),
// 			RequeueAfter: 30 * time.Second,
// 		}
// 	}

// 	for _, ip := range resp.Payload {
// 		currentEgressIPs.Insert(*ip.Ipaddress)
// 	}

// 	wantEgressIPs := sets.NewString()
// 	for _, egressRule := range r.infrastructureConfig.Firewall.EgressRules {
// 		wantEgressIPs.Insert(egressRule.IPs...)

// 		for _, ip := range egressRule.IPs {
// 			ip := ip
// 			if currentEgressIPs.Has(ip) {
// 				continue
// 			}

// 			resp, err := r.mclient.IP().FindIPs(metalip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
// 				Ipaddress: ip,
// 				Projectid: r.infrastructureConfig.ProjectID,
// 				Networkid: egressRule.NetworkID,
// 			}).WithContext(ctx), nil)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("error when retrieving ip %s for egress rule %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			switch len(resp.Payload) {
// 			case 0:
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("ip %s for egress rule does not exist", ip),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			case 1:
// 			default:
// 				return fmt.Errorf("ip %s found multiple times", ip)
// 			}

// 			dbIP := resp.Payload[0]
// 			if dbIP.Type != nil && *dbIP.Type != models.V1IPBaseTypeStatic {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("ips for egress rule must be static, but %s is not static", ip),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			if len(dbIP.Tags) > 0 {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("won't use ip %s for egress rules because it does not have an egress tag but it has other tags", *dbIP.Ipaddress),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			_, err = r.mclient.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(&models.V1IPUpdateRequest{
// 				Ipaddress: dbIP.Ipaddress,
// 				Tags:      []string{r.egressTag},
// 			}).WithContext(ctx), nil)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("could not tag ip %s for egress usage %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}
// 		}
// 	}

// 	if !currentEgressIPs.Equal(wantEgressIPs) {
// 		toUnTag := currentEgressIPs.Difference(wantEgressIPs)
// 		for _, ip := range toUnTag.List() {
// 			err := clearIPTags(ctx, r.mclient, ip)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("could not remove egress tag from ip %s %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

// func egressTag(clusterID string) string {
// 	return fmt.Sprintf("%s=%s", tag.ClusterEgress, clusterID)
// }
