package firewall

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/validation"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/models"

	metalgo "github.com/metal-stack/metal-go"
)

type (
	Config struct {
		Log logr.Logger
		ControllerConfig
	}
	ControllerConfig struct {
		Seed           client.Client
		Shoot          client.Client
		Metal          metalgo.Client
		Namespace      string
		ShootNamespace string
		ClusterID      string
		ClusterTag     string
		Recorder       record.EventRecorder
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
	if c.Metal == nil {
		return fmt.Errorf("metal client must be specified")
	}
	if c.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if c.ShootNamespace == "" {
		return fmt.Errorf("shoot namespace must be specified")
	}
	if c.ClusterID == "" {
		return fmt.Errorf("cluster id must be specified")
	}
	if c.ClusterTag == "" {
		return fmt.Errorf("cluster tag must be specified")
	}
	if c.Recorder == nil {
		return fmt.Errorf("recorder must be specified")
	}

	return nil
}

func (c *Config) SetupWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	g := controllers.NewGenericController[*v2.Firewall](c.Log, c.Seed, c.Namespace, &controller{
		ControllerConfig: &c.ControllerConfig,
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.Firewall{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})). // prevents reconcile on status sub resource update
		Named("Firewall").
		WithEventFilter(predicate.NewPredicateFuncs(controllers.SkipOtherNamespace(c.Namespace))).
		Complete(g)
}

func (c *Config) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if err := c.validate(); err != nil {
		return err
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&v2.Firewall{}).
		WithDefaulter(v2.NewFirewallDefaulter(c.Log.WithName("defaulting-webhook"))).
		WithValidator(validation.NewFirewallValidator(c.Log.WithName("validating-webhook"))).
		Complete()
}

func (c *controller) New() *v2.Firewall {
	return &v2.Firewall{}
}

func (c *controller) findAssociatedFirewalls(ctx context.Context, fw *v2.Firewall) ([]*models.V1FirewallResponse, error) {
	tags, err := c.firewallTags(fw)
	if err != nil {
		return nil, err
	}

	resp, err := c.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationName:    fw.Name,
		AllocationProject: fw.Spec.Project,
		Tags:              tags,
	}).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall find error: %w", err)
	}

	return resp.Payload, nil
}

func (c *controller) firewallTags(fw *v2.Firewall) ([]string, error) {
	ref := metav1.GetControllerOf(fw)
	if ref == nil {
		return nil, fmt.Errorf("firewall object has no owner reference")
	}

	return []string{c.ClusterTag, controllers.FirewallSetTag(ref.Name)}, nil
}
