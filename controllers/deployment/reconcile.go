package deployment

import (
	"context"
	"fmt"
	"strconv"
	"time"

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
		log.Error(err, "unable to ensure firewall controller rbac")

		cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionFalse, "Error", fmt.Sprintf("RBAC resources could not be provisioned %s", err))
		deploy.Status.Conditions.Set(cond)

		return err
	}

	cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, "Provisioned", "RBAC provisioned successfully.")
	deploy.Status.Conditions.Set(cond)

	switch s := deploy.Spec.Strategy; s {
	case v2.StrategyRecreate:
		err = c.recreateStrategy(ctx, log, deploy)
	case v2.StrategyRollingUpdate:
		err = c.rollingUpdateStrategy(ctx, log, deploy)
	default:
		err = fmt.Errorf("unknown deployment strategy: %s", s)
	}

	statusErr := c.setStatus(ctx, deploy)

	if err != nil {
		return err
	}
	if statusErr != nil {
		return err
	}

	log.Info("reonciling egress ips")
	err = c.reconcileEgressIPs(ctx, &deploy.Spec.Template)
	if err != nil {
		log.Error(err, "unable to reconcile egress ips")

		cond := v2.NewCondition(v2.FirewallDeplomentEgressIPs, v2.ConditionFalse, "Error", fmt.Sprintf("Egress IPs could not be reconciled: %s", err))
		deploy.Status.Conditions.Set(cond)

		return controllers.RequeueAfter(2*time.Minute, "backing off because egress ips can probably not be repaired by retrying")
	}

	var ips []string
	for _, ip := range deploy.Spec.Template.EgressRules {
		ips = append(ips, ip.IPs...)
	}
	cond = v2.NewCondition(v2.FirewallDeplomentEgressIPs, v2.ConditionTrue, "Reconciled", fmt.Sprintf("Egress IPs reconciled successfully. %v", ips))
	deploy.Status.Conditions.Set(cond)

	return nil
}

func (c *controller) createFirewallSet(ctx context.Context, log logr.Logger, deploy *v2.FirewallDeployment, revision int) (*v2.FirewallSet, error) {
	if lastCreation, ok := c.lastSetCreation[deploy.Name]; ok && time.Since(lastCreation) < c.safetyBackoff {
		// this is just for safety reasons to prevent mass-allocations
		log.Info("backing off from firewall set creation as last creation is only seconds ago", "ago", time.Since(lastCreation).String())
		return nil, controllers.RequeueAfter(10*time.Second, "delaying firewall set creation")
	}

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
			Annotations: map[string]string{
				controllers.RevisionAnnotation: strconv.Itoa(revision),
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

	c.lastSetCreation[deploy.Name] = time.Now()

	return set, nil
}

func (c *controller) deleteFirewallSets(ctx context.Context, log logr.Logger, sets []*v2.FirewallSet) error {
	for _, set := range sets {
		set := set

		err := c.Seed.Delete(ctx, set, &client.DeleteOptions{})
		if err != nil {
			return err
		}

		log.Info("deleted firewall set", "name", set.Name)

		c.Recorder.Eventf(set, "Normal", "Delete", "deleted firewallset %s", set.Name)
	}

	return nil
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
