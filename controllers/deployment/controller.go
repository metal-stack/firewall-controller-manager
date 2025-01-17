package deployment

import (
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
)

type controller struct {
	c               *config.ControllerConfig
	log             logr.Logger
	lastSetCreation map[string]time.Time
	recorder        record.EventRecorder
}

func SetupWithManager(log logr.Logger, recorder record.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		c:               c,
		log:             log,
		recorder:        recorder,
		lastSetCreation: map[string]time.Time{},
	})

	controller := ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallDeployment{},
			builder.WithPredicates(
				predicate.And(
					predicate.Not(v2.AnnotationAddedPredicate(v2.MaintenanceAnnotation)),
					predicate.Not(v2.AnnotationRemovedPredicate(v2.MaintenanceAnnotation)),
					predicate.Or(
						predicate.GenerationChangedPredicate{}, // prevents reconcile on status sub resource update
						predicate.AnnotationChangedPredicate{},
						predicate.LabelChangedPredicate{},
					),
				),
			),
		).
		Named("FirewallDeployment").
		Owns(
			&v2.FirewallSet{},
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
	defaulter, err := defaults.NewFirewallDeploymentDefaulter(log, c)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.FirewallDeployment{}).
		WithDefaulter(defaulter).
		WithValidator(validation.NewFirewallDeploymentValidator(log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.FirewallDeployment {
	return &v2.FirewallDeployment{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallDeployment, refetched *v2.FirewallDeployment) {
	refetched.Status = reconciled.Status
}
