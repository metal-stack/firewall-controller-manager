package defaults

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type (
	DefaulterConfig struct {
		Log         logr.Logger
		Seed        client.Client
		Namespace   string
		K8sVersion  *semver.Version
		ShootAccess *v2.ShootAccess
	}
	firewallDefaulter struct {
		*DefaulterConfig
	}
	firewallSetDefaulter struct {
		*DefaulterConfig
		fd *firewallDefaulter
	}
	firewallDeploymentDefaulter struct {
		*DefaulterConfig
		fd *firewallDefaulter
	}
)

func (c *DefaulterConfig) validate() error {
	if c.Seed == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.K8sVersion == nil {
		return fmt.Errorf("k8s version must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.ShootAccess == nil {
		return fmt.Errorf("shoot access must be specified")
	}

	return nil
}

func NewFirewallDefaulter(c *DefaulterConfig) (*firewallDefaulter, error) {
	err := c.validate()
	if err != nil {
		return nil, err
	}

	return &firewallDefaulter{DefaulterConfig: c}, nil
}

func NewFirewallSetDefaulter(c *DefaulterConfig) (admission.CustomDefaulter, error) {
	fd, err := NewFirewallDefaulter(c)
	if err != nil {
		return nil, err
	}

	return &firewallSetDefaulter{DefaulterConfig: c, fd: fd}, nil
}

func NewFirewallDeploymentDefaulter(c *DefaulterConfig) (admission.CustomDefaulter, error) {
	fd, err := NewFirewallDefaulter(c)
	if err != nil {
		return nil, err
	}

	return &firewallDeploymentDefaulter{DefaulterConfig: c, fd: fd}, nil
}

func (r *firewallDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*v2.Firewall)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.Log.Info("defaulting firewall resource", "name", f.GetName(), "namespace", f.GetNamespace())

	defaultFirewallSpec(&f.Spec)

	return nil
}

func (r *firewallSetDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	f, ok := obj.(*v2.FirewallSet)
	if !ok {
		return fmt.Errorf("mutator received unexpected type: %T", obj)
	}

	r.Log.Info("defaulting firewallset resource", "name", f.GetName(), "namespace", f.GetNamespace())

	if f.Spec.Replicas == 0 {
		f.Spec.Replicas = 1
	}
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

	r.Log.Info("defaulting firewalldeployment resource", "name", f.GetName(), "namespace", f.GetNamespace())

	if f.Spec.Replicas == 0 {
		f.Spec.Replicas = 1
	}
	if f.Spec.Strategy == "" {
		f.Spec.Strategy = v2.StrategyRollingUpdate
	}
	if f.Spec.Selector == nil {
		f.Spec.Selector = f.Spec.Template.Labels
	}

	defaultFirewallSpec(&f.Spec.Template.Spec)

	if f.Spec.Template.Spec.Userdata == "" {
		err := helper.EnsureFirewallControllerRBAC(ctx, r.K8sVersion, r.Seed, f, r.ShootAccess)
		if err != nil {
			return err
		}

		userdata, err := createUserdata(ctx, r.Seed, r.K8sVersion, r.Namespace, r.ShootAccess.APIServerURL)
		if err != nil {
			return err
		}

		if f.Annotations == nil {
			f.Annotations = map[string]string{
				v2.FirewallUserdataCompatibilityAnnotation: ">=v2.0.0",
			}
		} else {
			f.Annotations[v2.FirewallUserdataCompatibilityAnnotation] = ">=v2.0.0"
		}

		f.Spec.Template.Spec.Userdata = userdata
	}

	if len(f.Spec.Template.Spec.SSHPublicKeys) == 0 {
		key, err := r.getSSHPublicKey(ctx)
		if err != nil {
			return err
		}

		f.Spec.Template.Spec.SSHPublicKeys = []string{key}
	}

	return nil
}

func defaultFirewallSpec(f *v2.FirewallSpec) {
	if f.Interval == "" {
		f.Interval = "10s"
	}
}

func (f *firewallDeploymentDefaulter) getSSHPublicKey(ctx context.Context) (string, error) {
	sshSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.ShootAccess.SSHKeySecretName,
			Namespace: f.Namespace,
		},
	}
	err := f.Seed.Get(ctx, client.ObjectKeyFromObject(sshSecret), sshSecret)
	if err != nil {
		return "", fmt.Errorf("ssh secret not found: %w", err)
	}

	sshPublicKey, ok := sshSecret.Data["id_rsa.pub"]
	if !ok {
		return "", fmt.Errorf("ssh secret does not contain a public key")
	}

	return string(sshPublicKey), nil
}
