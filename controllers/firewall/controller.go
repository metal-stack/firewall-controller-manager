package firewall

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/cache"
)

type controller struct {
	c             *config.ControllerConfig
	log           logr.Logger
	recorder      record.EventRecorder
	networkCache  *cache.Cache[string, *models.V1NetworkResponse]
	firewallCache *cache.Cache[*v2.Firewall, []*models.V1FirewallResponse]
}

func SetupWithManager(log logr.Logger, recorder record.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		log:      log,
		recorder: recorder,
		c:        c,
		networkCache: cache.New(5*time.Minute, func(ctx context.Context, id string) (*models.V1NetworkResponse, error) {
			resp, err := c.GetMetal().Network().FindNetwork(network.NewFindNetworkParams().WithID(id).WithContext(ctx), nil)
			if err != nil {
				return nil, fmt.Errorf("network find error: %w", err)
			}
			return resp.Payload, nil
		}),
		// the cache is only very short but on quickly repeated status updates, this should prevent the metal-api from being flooded
		firewallCache: cache.New(5*time.Second, func(ctx context.Context, fw *v2.Firewall) ([]*models.V1FirewallResponse, error) {
			searchFirewalls := func() ([]*models.V1FirewallResponse, error) {
				resp, err := c.GetMetal().Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
					AllocationName:    fw.Name,
					AllocationProject: fw.Spec.Project,
					Tags:              []string{c.GetClusterTag()},
				}).WithContext(ctx), nil)
				if err != nil {
					return nil, fmt.Errorf("firewall search error: %w", err)
				}

				return resp.Payload, nil
			}

			// First try to find the firewall by machineID but check that allocation, project and hostname still matches
			// this prevent erroneous situations where a metal admin just deleted the allocated firewall by hand
			//
			// This is kind of an anti-pattern because we depend on our own status, but performance benefit of this approach is
			// big enough that we agreed to do it. We still need to run the expensive lookup in the metal-api in case deriving
			// the machine from the status field does not work.
			if fw.Status.MachineStatus != nil && fw.Status.MachineStatus.MachineID != "" {
				resp, err := c.GetMetal().Firewall().FindFirewall(firewall.NewFindFirewallParams().WithContext(ctx).WithID(fw.Status.MachineStatus.MachineID), nil)
				if err != nil {
					var defaultErr *firewall.FindFirewallDefault
					if errors.As(err, &defaultErr) && defaultErr.Code() == http.StatusNotFound {
						return searchFirewalls()
					}

					return nil, fmt.Errorf("firewall find error: %w", err)
				}

				if resp.Payload.Allocation != nil &&
					*resp.Payload.Allocation.Project == fw.Spec.Project &&
					*resp.Payload.Allocation.Hostname == fw.Name {
					return []*models.V1FirewallResponse{resp.Payload}, nil
				}
			}

			// in any other situations make a expensive find firewalls call
			return searchFirewalls()
		}),
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.Firewall{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{}, // prevents reconcile on status sub resource update
					predicate.AnnotationChangedPredicate{},
				),
			),
		).
		// don't think about owning the firewall monitor here, it's in the shoot cluster, we cannot watch two clusters with controller-runtime
		Named("Firewall").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace()))).
		Complete(g)
}

func SetupWebhookWithManager(log logr.Logger, mgr ctrl.Manager, c *config.ControllerConfig) error {
	defaulter, err := defaults.NewFirewallDefaulter(log, c)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.Firewall{}).
		WithDefaulter(defaulter).
		WithValidator(validation.NewFirewallValidator(log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.Firewall {
	return &v2.Firewall{}
}

func (c *controller) SetStatus(reconciled *v2.Firewall, refetched *v2.Firewall) {
	refetched.Status = reconciled.Status
}
