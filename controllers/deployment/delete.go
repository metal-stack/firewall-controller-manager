package deployment

import (
	"context"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Delete(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment) error {
	sets := v2.FirewallSetList{}
	err := c.Seed.List(ctx, &sets, client.InNamespace(c.Namespace))
	if err != nil {
		return err
	}

	for _, s := range sets.Items {
		s := s

		if !metav1.IsControlledBy(&s, deploy) {
			continue
		}

		log.Info("deleting firewall set", "name", s.Name)

		err = c.Seed.Delete(ctx, &s, &client.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
