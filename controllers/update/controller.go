package update

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	apiv2client "github.com/metal-stack/api/go/client"
	apiv2 "github.com/metal-stack/api/go/metalstack/api/v2"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/cache"
)

type controller struct {
	c          *config.ControllerConfig
	log        logr.Logger
	recorder   events.EventRecorder
	imageCache *cache.Cache[string, *apiv2.Image]
}

func SetupWithManager(log logr.Logger, recorder events.EventRecorder, mgr ctrl.Manager, c *config.ControllerConfig) error {
	g := controllers.NewGenericController(log, c.GetSeedClient(), c.GetSeedNamespace(), &controller{
		c:          c,
		log:        log,
		recorder:   recorder,
		imageCache: newImageCache(c.GetMetal()),
	}).WithoutStatus()

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&v2.FirewallDeployment{},
			builder.WithPredicates(
				v2.AnnotationAddedPredicate(v2.MaintenanceAnnotation),
			),
		).
		Named("Update").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.GetSeedNamespace()))).
		Complete(g)
}

func (c *controller) New() *v2.FirewallDeployment {
	return &v2.FirewallDeployment{}
}

func (c *controller) SetStatus(_ *v2.FirewallDeployment, _ *v2.FirewallDeployment) {}

func (c *controller) Delete(_ *controllers.Ctx[*v2.FirewallDeployment]) error {
	return nil
}

func newImageCache(m apiv2client.Client) *cache.Cache[string, *apiv2.Image] {
	return cache.New(5*time.Minute, func(ctx context.Context, id string) (*apiv2.Image, error) {
		resp, err := m.Apiv2().Image().Latest(ctx, &apiv2.ImageServiceLatestRequest{Os: id})
		if err != nil {
			return nil, err
		}

		return resp.Image, nil
	})
}
