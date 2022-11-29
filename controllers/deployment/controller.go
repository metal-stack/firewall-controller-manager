package deployment

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"

	metalgo "github.com/metal-stack/metal-go"
)

type (
	Config struct {
		Log logr.Logger
		ControllerConfig
	}
	ControllerConfig struct {
		Seed          client.Client
		Shoot         client.Client
		Metal         metalgo.Client
		K8sVersion    *semver.Version
		Namespace     string
		ClusterID     string
		ClusterTag    string
		ClusterAPIURL string
		Recorder      record.EventRecorder
	}

	controller struct {
		*ControllerConfig
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
	if c.K8sVersion == nil {
		return fmt.Errorf("k8s version must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.ClusterID == "" {
		return fmt.Errorf("cluster id must be specified")
	}
	if c.ClusterTag == "" {
		return fmt.Errorf("cluster tag must be specified")
	}
	if c.ClusterAPIURL == "" {
		return fmt.Errorf("cluster api url must be specified")
	}
	if c.Recorder == nil {
		return fmt.Errorf("recorder must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.FirewallDeployment](c.Log, c.Seed, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
	})

	err := ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallDeployment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})). // prevents reconcile on status sub resource update
		Named("FirewallDeployment").
		Owns(&v2.FirewallSet{}).
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.FirewallDeployment{}).
		WithValidator(validation.NewFirewallDeploymentValidator()).
		Complete()
}

func (c *controller) New() *v2.FirewallDeployment {
	return &v2.FirewallDeployment{}
}
