package set

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, set *v2.FirewallSet) error {
	err := set.Validate() // TODO: add a validating webhook to perform this validation immediately (https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
	if err != nil {
		return err
	}

	ownedFirewalls, err := controllers.GetOwnedResources(ctx, c.Seed, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	for _, fw := range ownedFirewalls {
		fw.Spec = set.Spec.Template

		err := c.Seed.Update(ctx, fw, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating firewall spec: %w", err)
		}
	}

	currentAmount := len(ownedFirewalls)

	if currentAmount < set.Spec.Replicas {
		for i := currentAmount; i < set.Spec.Replicas; i++ {
			fw, err := c.createFirewall(ctx, set)
			if err != nil {
				return err
			}
			log.Info("firewall created", "name", fw.Name)
			c.Recorder.Event(set, "Normal", "Create", fmt.Sprintf("created firewall %s", fw.Name))
		}
	}

	if currentAmount > set.Spec.Replicas {
		for i := set.Spec.Replicas; i < currentAmount; i++ {
			fw, err := c.deleteOneFirewall(ctx, ownedFirewalls)
			if err != nil {
				return err
			}
			log.Info("firewall deleted", "name", fw.Name)
			c.Recorder.Event(set, "Normal", "Delete", fmt.Sprintf("deleted firewall %s", fw.Name))
		}
	}

	return c.checkOrphans(ctx, log, set)
}

func (c *controller) deleteOneFirewall(ctx context.Context, ownedFirewalls []*v2.Firewall) (*v2.Firewall, error) {
	if len(ownedFirewalls) == 0 {
		return nil, fmt.Errorf("no firewall found for deletion")
	}

	// TODO: should we prefer deleting some firewalls over others?
	fw := ownedFirewalls[0]

	err := c.Seed.Delete(ctx, fw, &client.DeleteOptions{})
	if err != nil {
		return nil, err
	}

	return fw, nil
}

func (c *controller) createFirewall(ctx context.Context, set *v2.FirewallSet) (*v2.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := set.Namespace
	name := fmt.Sprintf("%s-firewall-%s", clusterName, uuid.String()[:5])

	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: set.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(set, v2.GroupVersion.WithKind("FirewallSet")),
			},
		},
		Spec:     set.Spec.Template,
		Userdata: set.Userdata,
	}

	// TODO: for backwards-compatibility create firewall object in the shoot cluster as well
	// maybe deploy v1 and create v2 api to manage in the new manner

	err = c.Seed.Create(ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}

func (c *controller) checkOrphans(ctx context.Context, log logr.Logger, set *v2.FirewallSet) error {
	resp, err := c.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: set.Spec.Template.ProjectID,
		Tags:              []string{c.ClusterTag, controllers.FirewallSetTag(set.Name)},
	}).WithContext(ctx), nil)
	if err != nil {
		return err
	}

	if len(resp.Payload) == 0 {
		return nil
	}

	fws := &v2.FirewallList{}
	err = c.Seed.List(ctx, fws, client.InNamespace(c.Namespace))
	if err != nil {
		return err
	}

	ownedFirewalls, err := controllers.GetOwnedResources(ctx, c.Seed, set, &v2.FirewallList{}, func(fl *v2.FirewallList) []*v2.Firewall {
		return fl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned firewalls: %w", err)
	}

	existingNames := map[string]bool{}
	for _, fw := range ownedFirewalls {
		existingNames[fw.Name] = true
	}

	for _, fw := range resp.Payload {
		if fw.Allocation == nil || fw.Allocation.Name == nil {
			continue
		}
		if _, ok := existingNames[*fw.Allocation.Name]; ok {
			continue
		}

		log.Info("found orphan firewall, deleting orphan", "name", *fw.Allocation.Name, "id", *fw.ID, "non-orphans", existingNames)

		_, err = c.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*fw.ID), nil)
		if err != nil {
			return fmt.Errorf("error deleting orphaned firewall: %w", err)
		}

		c.Recorder.Event(set, "Normal", "Delete", fmt.Sprintf("deleted orphaned firewall %s id %s", *fw.Allocation.Name, *fw.ID))
	}

	return nil
}
