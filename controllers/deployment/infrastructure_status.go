package deployment

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"sort"
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

func (c *controller) updateInfrastructureStatus(r *controllers.Ctx[*v2.FirewallDeployment], infrastructureName string, ownedFirewalls []*v2.Firewall) error {
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

	type infrastructure struct {
		Spec struct {
			ProviderConfig struct {
				Firewall struct {
					EgressRules []struct {
						IPs []string `json:"ips"`
					} `json:"egressRules"`
				} `json:"firewall"`
			} `json:"providerConfig"`
		} `json:"spec"`
		Status struct {
			EgressCIDRs []any `json:"egressCIDRs"`
		} `json:"status"`
	}

	infraRaw, err := json.Marshal(infraObj)
	if err != nil {
		return fmt.Errorf("unable to convert gardener infrastructure object: %w", err)
	}

	var typedInfra infrastructure
	err = json.Unmarshal(infraRaw, &typedInfra)
	if err != nil {
		return fmt.Errorf("unable to convert gardener infrastructure object: %w", err)
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

	for _, rule := range typedInfra.Spec.ProviderConfig.Firewall.EgressRules {
		for _, ip := range rule.IPs {
			parsed, err := netip.ParseAddr(ip)
			if err != nil {
				continue
			}

			egressCIDRs = append(egressCIDRs, fmt.Sprintf("%s/%d", ip, parsed.BitLen()))
		}
	}

	sortUntypedStringSlice(egressCIDRs)
	sortUntypedStringSlice(typedInfra.Status.EgressCIDRs)

	// check if an update is required or not
	if slices.Equal(egressCIDRs, typedInfra.Status.EgressCIDRs) {
		c.log.Info("found gardener infrastructure resource, egress cidrs already up-to-date", "infrastructure-name", infraObj.GetName(), "egress-cidrs", egressCIDRs)
		return nil
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

	aclObj := &unstructured.Unstructured{}

	aclObj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "extensions.gardener.cloud",
		Kind:    "Extension",
		Version: "v1alpha1",
	})

	err = c.c.GetSeedClient().Get(r.Ctx, client.ObjectKey{
		Namespace: c.c.GetSeedNamespace(),
		Name:      "acl",
	}, infraObj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	err = v2.AddAnnotation(r.Ctx, c.c.GetSeedClient(), aclObj, "gardener.cloud/operation", "reconcile")
	if err != nil {
		return fmt.Errorf("error annotating acl extension with reconcile operation: %w", err)
	}

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

func sortUntypedStringSlice(s []any) {
	sort.Slice(s, func(i, j int) bool {
		a, aok := s[i].(string)
		b, bok := s[j].(string)
		if aok && bok {
			return a < b
		}
		return false
	})
}
