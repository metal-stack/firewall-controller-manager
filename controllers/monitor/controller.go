package monitor

import (
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
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
	g := controllers.NewGenericController[*v2.FirewallMonitor](log, c.GetShootClient(), c.GetShootNamespace(), &controller{
		log: log,
		c:   c,
	}).WithoutStatus()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallMonitor{}).
		Named("FirewallMonitor").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetShootNamespace()))).
		WithEventFilter(v2.SkipRollSetAnnotationRemoval()).
		Complete(g)
}

func (c *controller) New() *v2.FirewallMonitor {
	return &v2.FirewallMonitor{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallMonitor, refetched *v2.FirewallMonitor) {}
