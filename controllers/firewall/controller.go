package firewall

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/cache"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"

	metalgo "github.com/metal-stack/metal-go"
)

type (
	Config struct {
		Log logr.Logger
		ControllerConfig
	}
	ControllerConfig struct {
		Seed                      client.Client
		Shoot                     client.Client
		Metal                     metalgo.Client
		Namespace                 string
		ShootNamespace            string
		ClusterTag                string
		APIServerURL              string
		ShootKubeconfigSecretName string
		ShootTokenSecretName      string
		SSHKeySecretName          string
		Recorder                  record.EventRecorder
	}

	controller struct {
		*ControllerConfig
		networkCache *cache.Cache[*models.V1NetworkResponse]
	}
)

func (c *Config) validate() error {
	if c.Seed == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.Shoot == nil {
		return fmt.Errorf("shoot client must be specified")
	}
	if c.Metal == nil {
		return fmt.Errorf("metal client must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.APIServerURL == "" {
		return fmt.Errorf("api server url must be specified")
	}
	if c.ShootNamespace == "" {
		return fmt.Errorf("shoot namespace must be specified")
	}
	if c.ClusterTag == "" {
		return fmt.Errorf("cluster tag must be specified")
	}
	if c.Recorder == nil {
		return fmt.Errorf("recorder must be specified")
	}
	if c.ShootKubeconfigSecretName == "" {
		return fmt.Errorf("shoot kubeconfig secret must be specified")
	}
	if c.ShootTokenSecretName == "" {
		return fmt.Errorf("shoot token secret name must be specified")
	}
	if c.SSHKeySecretName == "" {
		return fmt.Errorf("shoot ssh key secret name must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.Firewall](c.Log, c.Seed, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
		networkCache: cache.New(5*time.Minute, func(ctx context.Context, key any) (*models.V1NetworkResponse, error) {
			resp, err := c.Metal.Network().FindNetwork(network.NewFindNetworkParams().WithID(key.(string)).WithContext(ctx), nil)
			if err != nil {
				return nil, fmt.Errorf("network find error: %w", err)
			}
			return resp.Payload, nil
		}),
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.Firewall{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})). // prevents reconcile on status sub resource update
		Named("Firewall").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
}

func (c *Config) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.Firewall{}).
		WithDefaulter(v2.NewFirewallDefaulter(c.Log.WithName("defaulting-webhook"))).
		WithValidator(validation.NewFirewallValidator(c.Log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.Firewall {
	return &v2.Firewall{}
}

func (c *controller) findAssociatedFirewalls(ctx context.Context, fw *v2.Firewall) ([]*models.V1FirewallResponse, error) {
	resp, err := c.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationName:    fw.Name,
		AllocationProject: fw.Spec.Project,
		Tags:              []string{c.ClusterTag},
	}).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall find error: %w", err)
	}

	return resp.Payload, nil
}
