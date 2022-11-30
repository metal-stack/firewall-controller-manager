package firewall

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/models"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
	fws, err := c.findAssociatedFirewalls(ctx, fw)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	switch len(fws) {
	case 0:
		f, err := c.createFirewall(ctx, fw)
		if err != nil {
			log.Error(err, "error creating firewall")

			cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "Error", fmt.Sprintf("error creating firewall: %s", err))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(30*time.Second, "error creating firewall")
		}

		log.Info("firewall created", "id", pointer.SafeDeref(f.ID))

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
		fw.Status.Conditions.Set(cond)

		if err := c.setStatus(ctx, fw, f); err != nil {
			return err
		}

		// requeueing in order to continue checking progression
		return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")
	case 1:
		f := fws[0]

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
		fw.Status.Conditions.Set(cond)

		err := c.setStatus(ctx, fw, f)
		if err != nil {
			return err
		}

		if isFirewallReady(fw.Status.MachineStatus) {
			log.Info("firewall reconciled successfully", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionTrue, "Ready", fmt.Sprintf("firewall %q is phoning home and alive.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			return nil
		} else if isFirewallProgressing(fw.Status.MachineStatus) {
			log.Info("firewall is progressing", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotReady", fmt.Sprintf("firewall %q is not ready.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")
		} else {
			log.Error(fmt.Errorf("firewall is not finishing the provisioning"), "please investigate", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotFinishing", fmt.Sprintf("firewall %q is not finishing the provisioning procedure.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(1*time.Minute, "firewall creation is not finishing, proceed probing")
		}
	default:
		var ids []string
		for _, f := range fws {
			f := f
			ids = append(ids, pointer.SafeDeref(f.ID))
		}

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "MultipleFirewalls", fmt.Sprintf("found multiple firewalls with the same name: %s", strings.Join(ids, ", ")))
		fw.Status.Conditions.Set(cond)

		// TODO: should we just remove the other ones?

		return controllers.RequeueAfter(1*time.Minute, "multiple firewalls found with the same name, please investigate")
	}
}

func (c *controller) createFirewall(ctx context.Context, fw *v2.Firewall) (*models.V1FirewallResponse, error) {
	var (
		networks []*models.V1MachineAllocationNetwork
		tags     = []string{c.ClusterTag}
	)
	for _, n := range fw.Spec.Networks {
		n := n
		network := &models.V1MachineAllocationNetwork{
			Networkid:   &n,
			Autoacquire: pointer.Pointer(true),
		}
		networks = append(networks, network)
	}

	ref := metav1.GetControllerOf(fw)
	if ref != nil {
		tags = append(tags, controllers.FirewallSetTag(ref.Name))
	}

	createRequest := &models.V1FirewallCreateRequest{
		Description: "created by firewall-controller-manager",
		Name:        fw.Name,
		Hostname:    fw.Name,
		Sizeid:      &fw.Spec.Size,
		Projectid:   &fw.Spec.ProjectID,
		Partitionid: &fw.Spec.PartitionID,
		Imageid:     &fw.Spec.Image,
		SSHPubKeys:  fw.Spec.SSHPublicKeys,
		Networks:    networks,
		UserData:    fw.Userdata,
		Tags:        tags,
	}

	resp, err := c.Metal.Firewall().AllocateFirewall(firewall.NewAllocateFirewallParams().WithBody(createRequest).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall create error: %w", err)
	}

	c.Recorder.Eventf(fw, "Normal", "Create", "created firewall %s id %s", fw.Name, *resp.Payload.ID)

	return resp.Payload, nil
}

func isFirewallProgressing(status *v2.MachineStatus) bool {
	if status == nil || status.LastEvent == nil {
		return false
	}
	if status.CrashLoop {
		return false
	}
	if status.Liveliness != "Alive" {
		return false
	}
	if status.LastEvent.Event != "Phoned Home" {
		return true
	}
	return false
}

func isFirewallReady(status *v2.MachineStatus) bool {
	if status == nil || status.LastEvent == nil {
		return false
	}
	if status.CrashLoop {
		return false
	}
	if status.Liveliness != "Alive" {
		return false
	}
	if status.LastEvent.Event == "Phoned Home" {
		return true
	}
	return false
}
