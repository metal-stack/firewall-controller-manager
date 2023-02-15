package deployment

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
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
		Seed             client.Client
		SeedConfig       *rest.Config
		Metal            metalgo.Client
		ShootNamespace   string
		ShootAccess      *v2.ShootAccess
		Namespace        string
		Recorder         record.EventRecorder
		SafetyBackoff    time.Duration
		ProgressDeadline time.Duration
	}

	controller struct {
		*ControllerConfig
		lastSetCreation map[string]time.Time
	}
)

func (c *Config) validate() error {
	if c.Seed == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.SeedConfig == nil {
		return fmt.Errorf("seed config must be specified")
	}
	if c.Metal == nil {
		return fmt.Errorf("metal client must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.Recorder == nil {
		return fmt.Errorf("recorder must be specified")
	}
	if c.SafetyBackoff <= 0 {
		return fmt.Errorf("safety backoff must be specified")
	}
	if c.ProgressDeadline <= 0 {
		return fmt.Errorf("progress deadline must be specified")
	}
	if c.ShootNamespace == "" {
		return fmt.Errorf("shoot namespace must be specified")
	}
	if c.ShootAccess == nil {
		return fmt.Errorf("shoot access must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.FirewallDeployment](c.Log, c.Seed, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
		lastSetCreation:  map[string]time.Time{},
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallDeployment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})). // prevents reconcile on status sub resource update
		Named("FirewallDeployment").
		Owns(&v2.FirewallSet{}).
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
}

func (c *Config) SetupWebhookWithManager(mgr ctrl.Manager, dc *defaults.DefaulterConfig) error {
	if err := c.validate(); err != nil {
		return err
	}

	defaulter, err := defaults.NewFirewallDeploymentDefaulter(dc)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.FirewallDeployment{}).
		WithDefaulter(defaulter).
		WithValidator(validation.NewFirewallDeploymentValidator(c.Log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.FirewallDeployment {
	return &v2.FirewallDeployment{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallDeployment, refetched *v2.FirewallDeployment) {
	refetched.Status = reconciled.Status
}
