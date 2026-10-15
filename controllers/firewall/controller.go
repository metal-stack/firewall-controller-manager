package firewall

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/metal-stack/api/go/errorutil"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/cache"
)

type controller struct {
	c             *config.ControllerConfig
	log           logr.Logger
	recorder      events.EventRecorder
	networkCache  *cache.Cache[string, *apiv2.Network]
	firewallCache *cache.Cache[*v2.Firewall, []*apiv2.Machine]
}

func SetupWithManager(log logr.Logger, recorder events.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		log:      log,
		recorder: recorder,
		c:        c,
		networkCache: cache.New(5*time.Minute, func(ctx context.Context, id string) (*apiv2.Network, error) {
			resp, err := c.GetMetal().Apiv2().Network().Get(ctx, &apiv2.NetworkServiceGetRequest{Project: c.GetProject(), Id: id})
			if err != nil {
				return nil, fmt.Errorf("network find error: %w", err)
			}
			return resp.Network, nil
		}),
		// the cache is only very short but on quickly repeated status updates, this should prevent the metal-api from being flooded
		firewallCache: cache.New(5*time.Second, func(ctx context.Context, fw *v2.Firewall) ([]*apiv2.Machine, error) {
			searchFirewalls := func() ([]*apiv2.Machine, error) {
				resp, err := c.GetMetal().Apiv2().Machine().List(ctx, &apiv2.MachineServiceListRequest{
					Project: c.GetProject(),
					Query: &apiv2.MachineQuery{
						Allocation: &apiv2.MachineAllocationQuery{
							Name: &fw.Name,
							Labels: &apiv2.Labels{
								Labels: controllers.ToLabels([]string{c.GetClusterTag()}),
							},
						},
					},
				})
				if err != nil {
					return nil, fmt.Errorf("firewall search error: %w", err)
				}

				return resp.Machines, nil
			}

			// First try to find the firewall by machineID but check that allocation, project and hostname still matches
			// this prevent erroneous situations where a metal admin just deleted the allocated firewall by hand
			//
			// This is kind of an anti-pattern because we depend on our own status, but performance benefit of this approach is
			// big enough that we agreed to do it. We still need to run the expensive lookup in the metal-api in case deriving
			// the machine from the status field does not work.
			if fw.Status.MachineStatus != nil && fw.Status.MachineStatus.MachineID != "" {
				resp, err := c.GetMetal().Apiv2().Machine().Get(ctx, &apiv2.MachineServiceGetRequest{Project: c.GetProject(), Uuid: fw.Status.MachineStatus.MachineID})
				if err != nil {

					if errorutil.IsNotFound(err) {
						return searchFirewalls()
					}

					return nil, fmt.Errorf("firewall find error: %w", err)
				}

				if resp.Machine.Allocation != nil &&
					resp.Machine.Allocation.Project == fw.Spec.Project &&
					resp.Machine.Allocation.Hostname == fw.Name {
					return []*apiv2.Machine{resp.Machine}, nil
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

	return ctrl.NewWebhookManagedBy(mgr, &v2.Firewall{}).
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
