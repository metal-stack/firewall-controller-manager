package config

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	metalgo "github.com/metal-stack/metal-go"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NewControllerConfig struct {
	SeedClient       client.Client
	SeedConfig       *rest.Config
	SeedNamespace    string
	SeedAPIServerURL string

	ShootClient       client.Client
	ShootConfig       *rest.Config
	ShootNamespace    string
	ShootAPIServerURL string

	ShootAccess       *v2.ShootAccess
	ShootAccessHelper *helper.ShootAccessHelper

	Metal      metalgo.Client
	ClusterTag string

	SafetyBackoff         time.Duration
	ProgressDeadline      time.Duration
	FirewallHealthTimeout time.Duration
	CreateTimeout         time.Duration
}

type ControllerConfig struct {
	seedClient       client.Client
	seedConfig       *rest.Config
	seedNamespace    string
	seedAPIServerURL string

	shootClient       client.Client
	shootConfig       *rest.Config
	shootNamespace    string
	shootAPIServerURL string

	shootAccess               *v2.ShootAccess
	shootAccessHelper         *helper.ShootAccessHelper
	shootKubeconfigSecretName string
	shootTokenSecretName      string
	sshKeySecretName          string

	metal      metalgo.Client
	clusterTag string

	safetyBackoff         time.Duration
	progressDeadline      time.Duration
	firewallHealthTimeout time.Duration
	createTimeout         time.Duration
}

func New(c *NewControllerConfig) (*ControllerConfig, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	helper := helper.NewShootAccessHelper(c.SeedClient, c.ShootAccess)
	if c.ShootAccessHelper != nil {
		helper = c.ShootAccessHelper
	}

	return &ControllerConfig{
		seedClient:            c.SeedClient,
		seedConfig:            c.SeedConfig,
		seedNamespace:         c.SeedNamespace,
		seedAPIServerURL:      c.SeedAPIServerURL,
		shootClient:           c.ShootClient,
		shootConfig:           c.ShootConfig,
		shootNamespace:        c.ShootNamespace,
		shootAPIServerURL:     c.ShootAPIServerURL,
		shootAccess:           c.ShootAccess,
		shootAccessHelper:     helper,
		metal:                 c.Metal,
		clusterTag:            c.ClusterTag,
		safetyBackoff:         c.SafetyBackoff,
		progressDeadline:      c.ProgressDeadline,
		firewallHealthTimeout: c.FirewallHealthTimeout,
		createTimeout:         c.CreateTimeout,
	}, nil

}

func (c *NewControllerConfig) validate() error {
	if c.SeedClient == nil {
		return fmt.Errorf("seed client must be specified")
	}
	if c.SeedConfig == nil {
		return fmt.Errorf("seed config must be specified")
	}
	if c.SeedNamespace == "" {
		return fmt.Errorf("seed namespace must be specified")
	}
	if c.SeedAPIServerURL == "" {
		return fmt.Errorf("seed api server url must be specified")
	}

	if c.ShootClient == nil {
		return fmt.Errorf("shoot client must be specified")
	}
	if c.ShootConfig == nil {
		return fmt.Errorf("shoot config must be specified")
	}
	if c.ShootNamespace == "" {
		return fmt.Errorf("shoot namespace must be specified")
	}

	if c.ShootAccess == nil {
		return fmt.Errorf("shoot access must be specified")
	}
	if c.ShootAccess.GenericKubeconfigSecretName == "" {
		return fmt.Errorf("shoot kubeconfig secret must be specified")
	}
	if c.ShootAccess.TokenSecretName == "" {
		return fmt.Errorf("shoot token secret name must be specified")
	}
	if c.ShootAccess.SSHKeySecretName == "" {
		return fmt.Errorf("shoot ssh key secret name must be specified")
	}

	if c.Metal == nil {
		return fmt.Errorf("metal client must be specified")
	}
	if c.ClusterTag == "" {
		return fmt.Errorf("cluster tag must be specified")
	}

	if c.SafetyBackoff <= 0 {
		return fmt.Errorf("safety backoff must be specified")
	}
	if c.ProgressDeadline <= 0 {
		return fmt.Errorf("progress deadline must be specified")
	}
	if c.FirewallHealthTimeout <= 0 {
		return fmt.Errorf("firewall health timeout must be specified")
	}
	if c.CreateTimeout <= 0 {
		return fmt.Errorf("create timeout must be specified")
	}

	return nil
}

func (c *ControllerConfig) GetSeedClient() client.Client {
	return c.seedClient
}

func (c *ControllerConfig) GetSeedConfig() *rest.Config {
	return c.seedConfig
}

func (c *ControllerConfig) GetSeedNamespace() string {
	return c.seedNamespace
}

func (c *ControllerConfig) GetSeedAPIServerURL() string {
	return c.seedAPIServerURL
}

func (c *ControllerConfig) GetShootClient() client.Client {
	return c.shootClient
}

func (c *ControllerConfig) GetShootConfig() *rest.Config {
	return c.shootConfig
}

func (c *ControllerConfig) GetShootNamespace() string {
	return c.shootNamespace
}

func (c *ControllerConfig) GetShootAPIServerURL() string {
	return c.shootAPIServerURL
}

func (c *ControllerConfig) GetShootAccess() *v2.ShootAccess {
	return c.shootAccess
}

func (c *ControllerConfig) GetShootAccessHelper() *helper.ShootAccessHelper {
	return c.shootAccessHelper
}

func (c *ControllerConfig) GetShootKubeconfigSecretName() string {
	return c.shootKubeconfigSecretName
}

func (c *ControllerConfig) GetShootTokenSecretName() string {
	return c.shootTokenSecretName
}

func (c *ControllerConfig) GetSSHKeySecretName() string {
	return c.sshKeySecretName
}

func (c *ControllerConfig) GetMetal() metalgo.Client {
	return c.metal
}

func (c *ControllerConfig) GetClusterTag() string {
	return c.clusterTag
}

func (c *ControllerConfig) GetSafetyBackoff() time.Duration {
	return c.safetyBackoff
}

func (c *ControllerConfig) GetProgressDeadline() time.Duration {
	return c.progressDeadline
}

func (c *ControllerConfig) GetFirewallHealthTimeout() time.Duration {
	return c.firewallHealthTimeout
}

func (c *ControllerConfig) GetCreateTimeout() time.Duration {
	return c.createTimeout
}
