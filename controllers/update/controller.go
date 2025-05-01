package update

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/cache"
)

type controller struct {
	c          *config.ControllerConfig
	log        logr.Logger
	recorder   record.EventRecorder
	imageCache *cache.Cache[string, *models.V1ImageResponse]
}

func SetupWithManager(log logr.Logger, recorder record.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		c:          c,
		log:        log,
		recorder:   recorder,
		imageCache: newImageCache(c.GetMetal()),
	}).WithoutStatus()

	controller := ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallDeployment{},
			builder.WithPredicates(
				v2.AnnotationAddedPredicate(v2.MaintenanceAnnotation),
			),
		).
		Named("FirewallDeployment")

	if c.GetSeedNamespace() != "" {
		controller = controller.WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace())))
	}

	return controller.Complete(g)
}

func (c *controller) New() *v2.FirewallDeployment {
	return &v2.FirewallDeployment{}
}

func (c *controller) SetStatus(_ *v2.FirewallDeployment, _ *v2.FirewallDeployment) {}

func (c *controller) Delete(_ *controllers.Ctx[*v2.FirewallDeployment]) error {
	return nil
}

func newImageCache(m metalgo.Client) *cache.Cache[string, *models.V1ImageResponse] {
	return cache.New(5*time.Minute, func(ctx context.Context, id string) (*models.V1ImageResponse, error) {
		resp, err := m.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(id).WithContext(ctx), nil)
		if err != nil {
			return nil, err
		}

		return resp.Payload, nil
	})
}
