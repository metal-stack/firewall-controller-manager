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
	// SeedClient is used by the controllers to access the seed cluster.
	SeedClient client.Client
	// SeedConfig is the rest config used by the controllers to access the seed cluster.
	SeedConfig *rest.Config
	// SeedNamespace is the namespace within the seed cluster where the controllers act on.
	SeedNamespace string
	// SeedAPIServerURL is the endpoint of the seed cluster's api server. this is required
	// in order for the firewall-controller to access the seed cluster.
	SeedAPIServerURL string

	// ShootClient is used by the controllers to access the shoot cluster.
	ShootClient client.Client
	// ShootConfig is the rest config used by the controllers to access the shoot cluster.
	ShootConfig *rest.Config
	// ShootNamespace is the namespace within the shoot cluster where the controllers act on.
	ShootNamespace string
	// ShootAPIServerURL is the endpoint of the shoot cluster's api server. this is required
	// in order for the firewall-controller to access the shoot cluster.
	ShootAPIServerURL string

	// ShootAccess contains information for the firewall-controller to access the shoot cluster.
	// it is used by the firewall-controller to dynamically update rotating tokens for the accessing
	// the shoot cluster.
	ShootAccess *v2.ShootAccess
	// ShootAccessHelper wraps the shoot access and provides useful methods for dealing with
	// the shoot access secret rotation.
	ShootAccessHelper *helper.ShootAccessHelper

	// SSHKeySecretNamespace is the namespace that contains the ssh key secret in the seed cluster.
	SSHKeySecretNamespace string
	// SSHKeySecretName is the name of the ssh key secret in the seed cluster. it is used for
	// defaulting the ssh public keys when creating a new firewall.
	SSHKeySecretName string

	// Metal is the metal client for accessing the metal-api.
	Metal metalgo.Client
	// ClusterTag is the tag used in the metal-api for new firewalls to associate them with the cluster.
	ClusterTag string

	// SafetyBackoff is used for guarding the metal-api when it comes to creating new firewalls.
	SafetyBackoff time.Duration
	// ProgressDeadline is used to indicate unhealthy firewall deployment when it does not finish progressing.
	ProgressDeadline time.Duration
	// FirewallHealthTimeout is used to indicate an unhealthy firewall when it does not become ready.
	FirewallHealthTimeout time.Duration
	// CreateTimeout is used in the firewall creation phase to recreate a firewall when it does not become ready.
	CreateTimeout time.Duration

	// SkipValidation skips configuration validation, use this only for testing purposes
	SkipValidation bool
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

	sshKeySecretNamespace string
	sshKeySecretName      string

	metal      metalgo.Client
	clusterTag string

	safetyBackoff         time.Duration
	progressDeadline      time.Duration
	firewallHealthTimeout time.Duration
	createTimeout         time.Duration
}

func New(c *NewControllerConfig) (*ControllerConfig, error) {
	if err := c.validate(); !c.SkipValidation && err != nil {
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
		sshKeySecretNamespace: c.SSHKeySecretNamespace,
		sshKeySecretName:      c.SSHKeySecretName,
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
	if c.ShootAPIServerURL == "" {
		return fmt.Errorf("shoot api server url must be specified")
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

	if c.SSHKeySecretName == "" {
		return fmt.Errorf("shoot ssh key secret name must be specified")
	}
	if c.SSHKeySecretNamespace == "" {
		return fmt.Errorf("shoot ssh key secret namespace must be specified")
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
	if c.FirewallHealthTimeout < 0 {
		return fmt.Errorf("firewall health timeout must be specified")
	}
	if c.CreateTimeout < 0 {
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

func (c *ControllerConfig) GetSSHKeySecretNamespace() string {
	return c.sshKeySecretNamespace
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
