package monitor

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

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
		APIServerURL  string
		K8sVersion    *semver.Version
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
	if c.K8sVersion == nil {
		return fmt.Errorf("k8s version must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.SeedNamespace == "" {
		return fmt.Errorf("seed namespace must be specified")
	}
	if c.APIServerURL == "" {
		return fmt.Errorf("api server url must be specified")
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
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		WithEventFilter(v2.SkipRollSetAnnotationRemoval()).
		Complete(g)
}

func (c *controller) New() *v2.FirewallMonitor {
	return &v2.FirewallMonitor{}
}

func (c *controller) SetStatus(reconciled *v2.FirewallMonitor, refetched *v2.FirewallMonitor) {}
