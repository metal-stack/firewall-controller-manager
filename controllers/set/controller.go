package set

import (
	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type controller struct {
	log      logr.Logger
	recorder record.EventRecorder
	c        *config.ControllerConfig
}

func SetupWithManager(log logr.Logger, recorder record.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		log:      log,
		recorder: recorder,
		c:        c,
	})

	controller := ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallSet{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{}, // prevents reconcile on status sub resource update
					predicate.AnnotationChangedPredicate{},
				),
			),
		).
		Named("FirewallSet").
		Owns(
			&v2.Firewall{},
			builder.WithPredicates(
				predicate.Not(
					predicate.Or(
						v2.AnnotationAddedPredicate(v2.ReconcileAnnotation),
						v2.AnnotationRemovedPredicate(v2.ReconcileAnnotation),
					),
				),
			),
		)

	if c.GetSeedNamespace() != "" {
		controller = controller.WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace())))
	}

	return controller.Complete(g)
}

func SetupWebhookWithManager(log logr.Logger, mgr ctrl.Manager, c *config.ControllerConfig) error {
	defaulter, err := defaults.NewFirewallSetDefaulter(log, c)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.FirewallSet{}).
		WithDefaulter(defaulter).
		WithValidator(validation.NewFirewallSetValidator(log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.FirewallSet {
	return &v2.FirewallSet{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallSet, refetched *v2.FirewallSet) {
	refetched.Status = reconciled.Status
}
