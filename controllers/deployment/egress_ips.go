package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/ip"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TODO: probably it makes more sense to keep the egress ip logic in the GEPM

func (c *controller) reconcileEgressIPs(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	r.Log.Info("reonciling egress ips")

	var err error

	defer func() {
		if err != nil {
			r.Log.Error(err, "unable to reconcile egress ips")

			cond := v2.NewCondition(v2.FirewallDeplomentEgressIPs, v2.ConditionFalse, "Error", fmt.Sprintf("Egress IPs could not be reconciled: %s", err))
			r.Target.Status.Conditions.Set(cond)

			return
		}

		var ips []string
		for _, ip := range r.Target.Spec.Template.EgressRules {
			ips = append(ips, ip.IPs...)
		}
		cond := v2.NewCondition(v2.FirewallDeplomentEgressIPs, v2.ConditionTrue, "Reconciled", fmt.Sprintf("Egress IPs reconciled successfully. %v", ips))
		r.Target.Status.Conditions.Set(cond)
	}()

	fw := r.Target.Spec.Template

	resp, err := c.Metal.IP().FindIPs(ip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
		Projectid: fw.Project,
		Tags:      []string{egressTag(c.ClusterID)},
		Type:      models.V1IPBaseTypeStatic,
	}).WithContext(r.Ctx), nil)
	if err != nil {
		return fmt.Errorf("failed to list egress ips of cluster %w", err)
	}

	var (
		currentEgressIPs = sets.NewString()
		wantEgressIPs    = sets.NewString()
	)

	for _, ip := range resp.Payload {
		if ip.Ipaddress == nil {
			continue
		}
		currentEgressIPs.Insert(*ip.Ipaddress)
	}

	for _, egressRule := range fw.EgressRules {
		wantEgressIPs.Insert(egressRule.IPs...)

		for _, ipAddress := range egressRule.IPs {
			ipAddress := ipAddress
			if currentEgressIPs.Has(ipAddress) {
				continue
			}

			resp, err := c.Metal.IP().FindIPs(ip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
				Ipaddress: ipAddress,
				Projectid: fw.Project,
				Networkid: egressRule.NetworkID,
			}).WithContext(r.Ctx), nil)
			if err != nil {
				return fmt.Errorf("error when retrieving ip %s for egress rule %w", ipAddress, err)
			}

			switch len(resp.Payload) {
			case 0:
				return fmt.Errorf("ip %s for egress rule does not exist", ipAddress)
			case 1:
				// noop
			default:
				return fmt.Errorf("ip %s found multiple times", ipAddress)
			}

			dbIP := resp.Payload[0]
			if dbIP.Type != nil && *dbIP.Type != models.V1IPBaseTypeStatic {
				return fmt.Errorf("ips for egress rule must be static, but %s is not static", ipAddress)
			}

			if len(dbIP.Tags) > 0 {
				return fmt.Errorf("won't use ip %s for egress rules because it does not have an egress tag but it has other tags", *dbIP.Ipaddress)
			}

			_, err = c.Metal.IP().UpdateIP(ip.NewUpdateIPParams().WithBody(&models.V1IPUpdateRequest{
				Ipaddress: dbIP.Ipaddress,
				Tags:      []string{egressTag(c.ClusterID)},
			}).WithContext(r.Ctx), nil)
			if err != nil {
				return fmt.Errorf("could not tag ip %s for egress usage %w", ipAddress, err)
			}
		}
	}

	if !currentEgressIPs.Equal(wantEgressIPs) {
		toUnTag := currentEgressIPs.Difference(wantEgressIPs)
		for _, ipAddress := range toUnTag.List() {
			ipAddress := ipAddress

			_, err := c.Metal.IP().UpdateIP(ip.NewUpdateIPParams().WithBody(&models.V1IPUpdateRequest{
				Ipaddress: &ipAddress,
				Tags:      []string{},
			}).WithContext(r.Ctx), nil)
			if err != nil {
				return fmt.Errorf("could not remove egress tag from ip %s %w", ipAddress, err)
			}
		}
	}

	return nil
}

func egressTag(clusterID string) string {
	return fmt.Sprintf("%s=%s", tag.ClusterEgress, clusterID)
}
