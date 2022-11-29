package deployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment) error {
	log.Info("ensuring firewall controller rbac")
	err := c.ensureFirewallControllerRBAC(ctx)
	if err != nil {
		return err
	}

	ownedSets, err := controllers.GetOwnedResources(ctx, c.Seed, deploy, &v2.FirewallSetList{}, func(fsl *v2.FirewallSetList) []*v2.FirewallSet {
		return fsl.GetItems()
	})
	if err != nil {
		return fmt.Errorf("unable to get owned sets: %w", err)
	}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if lastSet == nil {
		log.Info("no firewall set is present, creating a new one")

		set, err := c.createFirewallSet(ctx, deploy)
		if err != nil {
			return fmt.Errorf("unable to create firewall set: %w", err)
		}

		c.Recorder.Eventf(set, "Normal", "Create", "created firewallset %s", set.Name)

		return nil
	}

	oldSets := controllers.Except(ownedSets, lastSet)

	log.Info("firewall sets already exist", "current-set-name", lastSet.Name, "old-set-count", len(oldSets))

	newSetRequired, err := c.isNewSetRequired(ctx, log, deploy, lastSet)
	if err != nil {
		return err
	}

	if !newSetRequired {
		log.Info("existing firewall set does not need to be rolled, only updating the resource")

		lastSet.Spec.Replicas = deploy.Spec.Replicas
		lastSet.Spec.Template = deploy.Spec.Template

		err = c.Seed.Update(ctx, lastSet, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update firewall set: %w", err)
		}

		log.Info("updated firewall set", "name", lastSet.Name)

		c.Recorder.Eventf(lastSet, "Normal", "Update", "updated firewallset %s", lastSet.Name)
	} else {
		// this is recreate strategy: TODO implement rolling update

		log.Info("significant changes detected in the spec, creating new firewall set")
		newSet, err := c.createFirewallSet(ctx, deploy)
		if err != nil {
			return err
		}

		log.Info("created new firewall set", "name", newSet.Name)

		oldSets = append(oldSets, lastSet.DeepCopy())

		c.Recorder.Eventf(newSet, "Normal", "Recreate", "recreated firewallset old: %s new: %s", lastSet.Name, newSet.Name)
	}

	for _, oldSet := range oldSets {
		log.Info("deleting old firewall sets")

		oldSet := oldSet
		err = c.Seed.Delete(ctx, oldSet, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		log.Info("deleted old firewall set", "name", oldSet.Name)

		c.Recorder.Eventf(oldSet, "Normal", "Delete", "deleted firewallset %s", oldSet.Name)
	}

	return nil
}

func (c *controller) createFirewallSet(ctx context.Context, deploy *v2.FirewallDeployment) (*v2.FirewallSet, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	userdata, err := c.createUserdata(ctx)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("%s-%s", deploy.Name, uuid.String()[:5])

	set := &v2.FirewallSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deploy.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(deploy, v2.GroupVersion.WithKind("FirewallDeployment")),
			},
		},
		Spec: v2.FirewallSetSpec{
			Replicas: deploy.Spec.Replicas,
			Template: deploy.Spec.Template,
		},
		Userdata: userdata,
	}

	err = c.Seed.Create(ctx, set, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall set: %w", err)
	}

	return set, nil
}

func (c *controller) isNewSetRequired(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment, lastSet *v2.FirewallSet) (bool, error) {
	ok := sizeHasChanged(deploy, lastSet)
	if ok {
		log.Info("firewall size has changed", "size", deploy.Spec.Template.Size)
		return ok, nil
	}

	ok, err := osImageHasChanged(ctx, c.Metal, deploy, lastSet)
	if err != nil {
		return false, err
	}
	if ok {
		log.Info("firewall image has changed", "image", deploy.Spec.Template.Image)
		return ok, nil
	}

	ok = networksHaveChanged(deploy, lastSet)
	if ok {
		log.Info("firewall networks have changed", "networks", deploy.Spec.Template.Networks)
		return ok, nil
	}

	return false, nil
}

func sizeHasChanged(deploy *v2.FirewallDeployment, lastSet *v2.FirewallSet) bool {
	return lastSet.Spec.Template.Size != deploy.Spec.Template.Size
}

func osImageHasChanged(ctx context.Context, m metalgo.Client, deploy *v2.FirewallDeployment, lastSet *v2.FirewallSet) (bool, error) {
	if lastSet.Spec.Template.Image != deploy.Spec.Template.Image {
		want := deploy.Spec.Template.Image
		image, err := m.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(want).WithContext(ctx), nil)
		if err != nil {
			return false, fmt.Errorf("latest firewall image not found:%s %w", want, err)
		}

		if image.Payload != nil && image.Payload.ID != nil && *image.Payload.ID != lastSet.Spec.Template.Image {
			return true, nil
		}
	}

	return false, nil
}

func networksHaveChanged(deploy *v2.FirewallDeployment, lastSet *v2.FirewallSet) bool {
	currentNetworks := sets.NewString()
	for _, n := range lastSet.Spec.Template.Networks {
		currentNetworks.Insert(n)
	}
	wantNetworks := sets.NewString()
	for _, n := range deploy.Spec.Template.Networks {
		wantNetworks.Insert(n)
	}

	return !currentNetworks.Equal(wantNetworks)
}
