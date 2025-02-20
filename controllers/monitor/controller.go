package monitor

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

type controller struct {
	log logr.Logger
	c   *config.ControllerConfig
}

func SetupWithManager(log logr.Logger, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetShootClient(), c.GetShootNamespace(), &controller{
		log: log,
		c:   c,
	}).WithoutStatus()

	controller := ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallMonitor{},
			builder.WithPredicates(
				predicate.Not(
					v2.AnnotationRemovedPredicate(v2.RollSetAnnotation),
				),
			),
		).
		Named("FirewallMonitor")

	if c.GetSeedNamespace() != "" {
		controller = controller.WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace())))
	}

	return controller.Complete(g)
}

func (c *controller) New() *v2.FirewallMonitor {
	return &v2.FirewallMonitor{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallMonitor, refetched *v2.FirewallMonitor) {}
