package defaults

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	DefaultFirewallReconcileInterval = "10s"
)

type (
	firewallDefaulter struct {
		c   *config.ControllerConfig
		log logr.Logger
	}
	firewallSetDefaulter struct {
		c   *config.ControllerConfig
		fd  *firewallDefaulter
		log logr.Logger
	}
	firewallDeploymentDefaulter struct {
		c   *config.ControllerConfig
		fd  *firewallDefaulter
		log logr.Logger
	}
)

func NewFirewallDefaulter(log logr.Logger, c *config.ControllerConfig) (*firewallDefaulter, error) {
	return &firewallDefaulter{log: log, c: c}, nil
}

func NewFirewallSetDefaulter(log logr.Logger, c *config.ControllerConfig) (admission.CustomDefaulter, error) {
	fd, err := NewFirewallDefaulter(log, c)
	if err != nil {
		return nil, err
	}

	return &firewallSetDefaulter{log: log, c: c, fd: fd}, nil
}

func NewFirewallDeploymentDefaulter(log logr.Logger, c *config.ControllerConfig) (admission.CustomDefaulter, error) {
	fd, err := NewFirewallDefaulter(log, c)
	if err != nil {
		return nil, err
	}

	return &firewallDeploymentDefaulter{log: log, c: c, fd: fd}, nil
}

func (r *firewallDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*v2.Firewall)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewall resource", "name", f.GetName(), "namespace", f.GetNamespace())

	defaultFirewallSpec(&f.Spec)

	return nil
}

func (r *firewallSetDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*v2.FirewallSet)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewallset resource", "name", f.GetName(), "namespace", f.GetNamespace())

	if f.Spec.Selector == nil {
		f.Spec.Selector = f.Spec.Template.Labels
	}

	defaultFirewallSpec(&f.Spec.Template.Spec)

	return nil
}

func (r *firewallDeploymentDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*v2.FirewallDeployment)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewalldeployment resource", "name", f.GetName(), "namespace", f.GetNamespace())

	if f.Spec.Strategy == "" {
		f.Spec.Strategy = v2.StrategyRollingUpdate
	}
	if f.Spec.Selector == nil {
		f.Spec.Selector = f.Spec.Template.Labels
	}

	defaultFirewallSpec(&f.Spec.Template.Spec)

	if f.Spec.Template.Spec.Userdata == "" {
		shootConfig, err := r.c.GetShootAccessHelper().RESTConfig(ctx)
		if err != nil {
			return err
		}

		err = helper.EnsureFirewallControllerRBAC(ctx, r.c.GetSeedConfig(), shootConfig, f, r.c.GetShootNamespace(), r.c.GetShootAccess())
		if err != nil {
			return err
		}

		shootKubeconfig, err := helper.GetAccessKubeconfig(&helper.AccessConfig{
			Ctx:          ctx,
			Config:       shootConfig,
			Namespace:    r.c.GetShootNamespace(),
			ApiServerURL: r.c.GetShootAPIServerURL(),
			Deployment:   f,
			ForShoot:     true,
		})
		if err != nil {
			return fmt.Errorf("error creating raw shoot kubeconfig: %w", err)
		}

		seedKubeconfig, err := helper.GetAccessKubeconfig(&helper.AccessConfig{
			Ctx:          ctx,
			Config:       r.c.GetSeedConfig(),
			Namespace:    r.c.GetSeedNamespace(),
			ApiServerURL: r.c.GetSeedAPIServerURL(),
			Deployment:   f,
		})
		if err != nil {
			return fmt.Errorf("error creating raw seed kubeconfig: %w", err)
		}

		userdata, err := renderUserdata(shootKubeconfig, seedKubeconfig)
		if err != nil {
			return err
		}

		f.Spec.Template.Spec.Userdata = userdata
	}

	if len(f.Spec.Template.Spec.SSHPublicKeys) == 0 {
		key, err := helper.GetSSHPublicKey(ctx, r.c.GetSeedClient(), r.c.GetShootAccess())
		if err != nil {
			return err
		}

		f.Spec.Template.Spec.SSHPublicKeys = []string{key}
	}

	return nil
}

func defaultFirewallSpec(f *v2.FirewallSpec) {
	if f.Interval == "" {
		f.Interval = DefaultFirewallReconcileInterval
	}
}
