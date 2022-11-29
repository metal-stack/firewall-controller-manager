package firewall

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (c *controller) Reconcile(ctx context.Context, log logr.Logger, fw *v2.Firewall) error {
	fws, err := c.findAssociatedFirewalls(ctx, fw)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	switch len(fws) {
	case 0:
		_, err := c.createFirewall(ctx, fw)
		if err != nil {
			return err
		}
		return controllers.StillProgressing()
	case 1:
		f := fws[0]
		if isFirewallReady(f) {
			log.Info("firewall is phoning home")
			return nil
		} else if isFirewallProgressing(f) {
			return controllers.StillProgressing()
		}
		return fmt.Errorf("firewall is not finishing")
	default:
		return fmt.Errorf("multiple firewalls found")
	}
}

func isFirewallProgressing(fw *models.V1FirewallResponse) bool {
	if fw.Events == nil || len(fw.Events.Log) == 0 || fw.Events.Log[0].Event == nil {
		return false
	}
	if fw.Events.CrashLoop == nil || *fw.Events.CrashLoop {
		return false
	}
	if fw.Events.FailedMachineReclaim == nil || *fw.Events.FailedMachineReclaim {
		return false
	}
	if *fw.Events.Log[0].Event != "Phoned Home" {
		return true
	}
	return false
}

func isFirewallReady(fw *models.V1FirewallResponse) bool {
	if fw.Events == nil || len(fw.Events.Log) == 0 || fw.Events.Log[0].Event == nil {
		return false
	}
	if fw.Events.CrashLoop == nil || *fw.Events.CrashLoop {
		return false
	}
	if fw.Events.FailedMachineReclaim == nil || *fw.Events.FailedMachineReclaim {
		return false
	}
	if *fw.Events.Log[0].Event == "Phoned Home" {
		return true
	}
	return false
}

func (c *controller) createFirewall(ctx context.Context, fw *v2.Firewall) (*models.V1FirewallResponse, error) {
	var networks []*models.V1MachineAllocationNetwork
	for _, n := range fw.Spec.Networks {
		n := n
		network := &models.V1MachineAllocationNetwork{
			Networkid:   &n,
			Autoacquire: pointer.Bool(true),
		}
		networks = append(networks, network)
	}

	ref := metav1.GetControllerOf(fw)
	if ref == nil {
		return nil, fmt.Errorf("firewall object has no owner reference")
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
		Tags:        []string{c.ClusterTag, controllers.FirewallSetTag(ref.Name)},
	}

	resp, err := c.Metal.Firewall().AllocateFirewall(firewall.NewAllocateFirewallParams().WithBody(createRequest).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall create error: %w", err)
	}

	c.Recorder.Eventf(fw, "Normal", "Create", "created firewall %s id %s", fw.Name, *resp.Payload.ID)

	return resp.Payload, nil
}
