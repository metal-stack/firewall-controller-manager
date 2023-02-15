package defaults

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type (
	DefaulterConfig struct {
		Log               logr.Logger
		SeedClient        client.Client
		SeedConfig        *rest.Config
		SeedNamespace     string
		ShootNamespace    string
		ShootAPIServerURL string
		SeedAPIServerURL  string
		ShootAccess       *v2.ShootAccess
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
	if c.SeedClient == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.SeedConfig == nil {
		return fmt.Errorf("seed config must be specified")
	}
	if c.SeedNamespace == "" {
		return fmt.Errorf("seed namespace must be specified")
	}
	if c.ShootNamespace == "" {
		return fmt.Errorf("shoot namespace must be specified")
	}
	if c.ShootAccess == nil {
		return fmt.Errorf("shoot access must be specified")
	}
	if c.SeedAPIServerURL == "" {
		return fmt.Errorf("seed api server url must be specified")
	}
	if c.ShootAPIServerURL == "" {
		return fmt.Errorf("shoot api server url must be specified")
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
		err := helper.EnsureFirewallControllerRBAC(ctx, r.SeedConfig, f, r.ShootNamespace, r.ShootAccess)
		if err != nil {
			return err
		}

		_, _, shootConfig, err := helper.NewShootConfig(ctx, r.SeedClient, r.ShootAccess)
		if err != nil {
			return err
		}

		shootKubeconfig, err := helper.GetAccessKubeconfig(&helper.AccessConfig{
			Ctx:          ctx,
			Config:       shootConfig,
			Namespace:    r.ShootNamespace,
			ApiServerURL: r.ShootAPIServerURL,
			Deployment:   f,
		})
		if err != nil {
			return err
		}

		seedKubeconfig, err := helper.GetAccessKubeconfig(&helper.AccessConfig{
			Ctx:          ctx,
			Config:       r.SeedConfig,
			Namespace:    r.SeedNamespace,
			ApiServerURL: r.SeedAPIServerURL,
			Deployment:   f,
		})
		if err != nil {
			return err
		}

		userdata, err := renderUserdata(shootKubeconfig, seedKubeconfig)
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
			Namespace: f.SeedNamespace,
		},
	}
	err := f.SeedClient.Get(ctx, client.ObjectKeyFromObject(sshSecret), sshSecret)
	if err != nil {
		return "", fmt.Errorf("ssh secret not found: %w", err)
	}

	sshPublicKey, ok := sshSecret.Data["id_rsa.pub"]
	if !ok {
		return "", fmt.Errorf("ssh secret does not contain a public key")
	}

	return string(sshPublicKey), nil
}
