package deployment

import (
	"context"
	"fmt"
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
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/cache"
)

type controller struct {
	c               *config.ControllerConfig
	log             logr.Logger
	lastSetCreation map[string]time.Time
	recorder        record.EventRecorder
	imageCache      *cache.Cache[string, *models.V1ImageResponse]
}

func SetupWithManager(log logr.Logger, recorder record.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController[*v2.FirewallDeployment](log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		c:               c,
		log:             log,
		recorder:        recorder,
		lastSetCreation: map[string]time.Time{},
		imageCache: cache.New(5*time.Minute, func(ctx context.Context, id string) (*models.V1ImageResponse, error) {
			resp, err := c.GetMetal().Image().FindLatestImage(image.NewFindLatestImageParams().WithID(id).WithContext(ctx), nil)
			if err != nil {
				return nil, fmt.Errorf("latest firewall image %q not found: %w", id, err)
			}

			return resp.Payload, nil
		}),
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallDeployment{},
			builder.WithPredicates(
				predicate.Or(
					predicate.GenerationChangedPredicate{}, // prevents reconcile on status sub resource update
					predicate.AnnotationChangedPredicate{},
					predicate.LabelChangedPredicate{},
				),
			),
		).
		Named("FirewallDeployment").
		Owns(&v2.FirewallSet{}).
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace()))).
		Complete(g)
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
