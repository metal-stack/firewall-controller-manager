package timeout

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

type controller struct {
	c        *config.ControllerConfig
	log      logr.Logger
	recorder events.EventRecorder
}

func SetupWithManager(log logr.Logger, recorder events.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	if c.GetFirewallHealthTimeout() <= 0 && c.GetCreateTimeout() <= 0 {
		log.Info("not registering timeout controller because neither create nor health timeout configured")
		return nil
	}

	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		c:        c,
		log:      log,
		recorder: recorder,
	}).WithoutStatus()

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallSet{},
		).
		Named("FirewallHealthTimeout").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace()))).
		Complete(g)
}

func (c *controller) New() *v2.FirewallSet {
	return &v2.FirewallSet{}
}

func (c *controller) SetStatus(_ *v2.FirewallSet, _ *v2.FirewallSet) {}

func (c *controller) Delete(_ *controllers.Ctx[*v2.FirewallSet]) error {
	return nil
}
