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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
	defer func() {
		_, err := c.ensureFirewallMonitor(ctx, fw)
		if err != nil {
			log.Error(err, "unable to deploy firewall monitor")

			cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionFalse, "Error", fmt.Sprintf("Monitor could not be deployed: %s", err))
			fw.Status.Conditions.Set(cond)

			return
		}

		log.Info("firewall monitor deployed")

		cond := v2.NewCondition(v2.FirewallMonitorDeployed, v2.ConditionTrue, "Deployed", "Successfully deployed firewall-monitor.")
		fw.Status.Conditions.Set(cond)

		return
	}()

	fws, err := c.findAssociatedFirewalls(ctx, fw)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	switch len(fws) {
	case 0:
		f, err := c.createFirewall(ctx, fw)
		if err != nil {
			log.Error(err, "error creating firewall")

			cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "Error", fmt.Sprintf("Error creating firewall: %s", err))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(30*time.Second, "error creating firewall")
		}

		log.Info("firewall created", "id", pointer.SafeDeref(f.ID))

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("Firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
		fw.Status.Conditions.Set(cond)

		if err := c.setStatus(ctx, fw, f); err != nil {
			return err
		}

		// requeueing in order to continue checking progression
		return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")
	case 1:
		f := fws[0]

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionTrue, "Created", fmt.Sprintf("Firewall %q created successfully.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
		fw.Status.Conditions.Set(cond)

		err := c.setStatus(ctx, fw, f)
		if err != nil {
			return err
		}

		if isFirewallReady(fw.Status.MachineStatus) {
			log.Info("firewall reconciled successfully", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionTrue, "Ready", fmt.Sprintf("Firewall %q is phoning home and alive.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			// to make the controller always sync the status with the metal-api, we requeue
			return controllers.RequeueAfter(1*time.Minute, "firewall creation succeeded, continue probing regularly")
		} else if isFirewallProgressing(fw.Status.MachineStatus) {
			log.Info("firewall is progressing", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotReady", fmt.Sprintf("Firewall %q is not ready.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(10*time.Second, "firewall creation is progressing")
		} else {
			log.Error(fmt.Errorf("firewall is not finishing the provisioning"), "please investigate", "id", pointer.SafeDeref(f.ID))

			cond := v2.NewCondition(v2.FirewallReady, v2.ConditionFalse, "NotFinishing", fmt.Sprintf("Firewall %q is not finishing the provisioning procedure.", pointer.SafeDeref(pointer.SafeDeref(f.Allocation).Name)))
			fw.Status.Conditions.Set(cond)

			return controllers.RequeueAfter(1*time.Minute, "firewall creation is not finishing, proceed probing")
		}
	default:
		var ids []string
		for _, f := range fws {
			f := f
			ids = append(ids, pointer.SafeDeref(f.ID))
		}

		cond := v2.NewCondition(v2.FirewallCreated, v2.ConditionFalse, "MultipleFirewalls", fmt.Sprintf("Found multiple firewalls with the same name: %s", strings.Join(ids, ", ")))
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

func (c *controller) ensureFirewallMonitor(ctx context.Context, fw *v2.Firewall) (*v2.FirewallMonitor, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.ShootNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c.Shoot, ns, func() error {
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure namespace for monitor resource: %w", err)
	}

	mon := &v2.FirewallMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fw.Name,
			Namespace: c.ShootNamespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, c.Shoot, mon, func() error {
		mon.Size = fw.Spec.Size
		mon.Image = fw.Spec.Image
		mon.PartitionID = fw.Spec.PartitionID
		mon.ProjectID = fw.Spec.ProjectID
		mon.Networks = fw.Spec.Networks
		mon.RateLimits = fw.Spec.RateLimits
		mon.EgressRules = fw.Spec.EgressRules
		mon.LogAcceptedConnections = fw.Spec.LogAcceptedConnections
		mon.MachineStatus = fw.Status.MachineStatus
		mon.ControllerStatus = fw.Status.ControllerStatus
		mon.Conditions = fw.Status.Conditions
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unable to ensure firewall monitor resource: %w", err)
	}

	return mon, nil
}
