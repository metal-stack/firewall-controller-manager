package monitor

import (
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
)

type (
	Config struct {
		Log logr.Logger
		ControllerConfig
	}
	ControllerConfig struct {
		Seed          client.Client
		Shoot         client.Client
		Namespace     string
		SeedNamespace string
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
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.SeedNamespace == "" {
		return fmt.Errorf("seed namespace must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.FirewallMonitor](c.Log, c.Shoot, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
	}).WithoutStatus()

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallMonitor{}).
		Named("FirewallMonitor").
		// WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
}

func (c *controller) New() *v2.FirewallMonitor {
	return &v2.FirewallMonitor{}
}
