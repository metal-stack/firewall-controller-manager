package set

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) updateInfrastructureStatus(r *controllers.Ctx[*v2.FirewallSet], ownedFirewalls []*v2.Firewall) error {
	infrastructureName, ok := extractInfrastructureNameFromSeedNamespace(c.c.GetSeedNamespace())
	if !ok {
		return nil
	}

	infraObj := &unstructured.Unstructured{}

	infraObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "extensions.gardener.cloud",
		Kind:    "Infrastructure",
		Version: "v1alpha1",
	})

	err := c.c.GetSeedClient().Get(r.Ctx, client.ObjectKey{
		Namespace: c.c.GetSeedNamespace(),
		Name:      infrastructureName,
	}, infraObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	var egressCIDRs []any

	for _, fw := range ownedFirewalls {
		for _, network := range fw.Status.FirewallNetworks {
			if pointer.SafeDeref(network.NetworkType) != "external" {
				continue
			}

			for _, ip := range network.IPs {
				parsed, err := netip.ParseAddr(ip)
				if err != nil {
					continue
				}

				egressCIDRs = append(egressCIDRs, fmt.Sprintf("%s/%d", ip, parsed.BitLen()))
			}
		}
	}

	// check if an update is required or not
	if currentStatus, ok := infraObj.Object["status"].(map[string]any); ok {
		if currentCIDRs, ok := currentStatus["egressCIDRs"].([]any); ok {
			if slices.Equal(egressCIDRs, currentCIDRs) {
				c.log.Info("found gardener infrastructure resource, egress cidrs already up-to-date", "infrastructure-name", infraObj.GetName(), "egress-cidrs", egressCIDRs)
				return nil
			}
		}
	}

	infraStatusPatch := map[string]any{
		"status": map[string]any{
			"egressCIDRs": egressCIDRs,
		},
	}

	jsonPatch, err := json.Marshal(infraStatusPatch)
	if err != nil {
		return fmt.Errorf("unable to marshal infrastructure status patch: %w", err)
	}

	err = c.c.GetSeedClient().Status().Patch(r.Ctx, infraObj, client.RawPatch(types.MergePatchType, jsonPatch))
	if err != nil {
		return fmt.Errorf("error patching infrastructure status egress cidrs field: %w", err)
	}

	c.log.Info("found gardener infrastructure resource and patched egress cidrs for acl extension", "infrastructure-name", infraObj.GetName(), "egress-cidrs", egressCIDRs)

	return nil
}

func extractInfrastructureNameFromSeedNamespace(namespace string) (string, bool) {
	if !strings.HasPrefix(namespace, "shoot--") {
		return "", false
	}

	parts := strings.Split(namespace, "--")
	if len(parts) < 3 {
		return "", false
	}

	return strings.Join(parts[2:], "--"), true
}
