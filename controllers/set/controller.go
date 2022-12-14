package set

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metalgo "github.com/metal-stack/metal-go"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type (
	Config struct {
		Log logr.Logger
		ControllerConfig
	}
	ControllerConfig struct {
		Seed                  client.Client
		Metal                 metalgo.Client
		Namespace             string
		ClusterTag            string
		FirewallHealthTimeout time.Duration
		CreateTimeout         time.Duration
		Recorder              record.EventRecorder
	}

	controller struct {
		*ControllerConfig
	}
)

func (c *Config) validate() error {
	if c.Seed == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.Metal == nil {
		return fmt.Errorf("metal client must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.ClusterTag == "" {
		return fmt.Errorf("cluster tag must be specified")
	}
	if c.Recorder == nil {
		return fmt.Errorf("recorder must be specified")
	}
	if c.CreateTimeout <= 0 {
		return fmt.Errorf("create timeout must be specified")
	}
	if c.FirewallHealthTimeout <= 0 {
		return fmt.Errorf("firewall health timeout must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.FirewallSet](c.Log, c.Seed, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})). // prevents reconcile on status sub resource update
		Named("FirewallSet").
		Owns(&v2.Firewall{}).
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
}

func (c *Config) SetupWebhookWithManager(mgr ctrl.Manager, dc *defaults.DefaulterConfig) error {
	if err := c.validate(); err != nil {
		return err
	}

	defaulter, err := defaults.NewFirewallSetDefaulter(dc)
	if err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.FirewallSet{}).
		WithDefaulter(defaulter).
		WithValidator(validation.NewFirewallSetValidator(c.Log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.FirewallSet {
	return &v2.FirewallSet{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallSet, refetched *v2.FirewallSet) {
	refetched.Status = reconciled.Status
}
