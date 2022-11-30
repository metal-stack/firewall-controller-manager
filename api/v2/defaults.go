package v2

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type (
	firewallDefaulter struct {
		log logr.Logger
	}
	firewallSetDefaulter struct {
		log logr.Logger
	}
	firewallDeploymentDefaulter struct {
		log logr.Logger
	}
)

func NewFirewallDefaulter(log logr.Logger) admission.CustomDefaulter {
	return &firewallDefaulter{log: log}
}

func (r *firewallDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*Firewall)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewall resource", "name", f.GetName(), "namespace", f.GetNamespace())

	f.Spec.Default()

	return nil
}

func NewFirewallSetDefaulter(log logr.Logger) admission.CustomDefaulter {
	return &firewallSetDefaulter{log: log}
}

func (r *firewallSetDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*FirewallSet)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewallset resource", "name", f.GetName(), "namespace", f.GetNamespace())

	f.Spec.Template.Default()

	return nil
}

func NewFirewallDeploymentDefaulter(log logr.Logger) admission.CustomDefaulter {
	return &firewallDeploymentDefaulter{log: log}
}

func (r *firewallDeploymentDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*FirewallDeployment)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.log.Info("defaulting firewalldeployment resource", "name", f.GetName(), "namespace", f.GetNamespace())

	if f.Spec.Strategy == "" {
		f.Spec.Strategy = StrategyRollingUpdate
	}

	f.Spec.Template.Default()

	return nil
}

func (f *FirewallSpec) Default() {
	if f.Interval == "" {
		f.Interval = "10s"
	}
}
