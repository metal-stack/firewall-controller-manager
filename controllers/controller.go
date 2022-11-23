/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"
	mn "github.com/metal-stack/metal-lib/pkg/net"

	v1 "github.com/metal-stack/firewall-controller/api/v1"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed       client.Client
	Shoot      client.Client
	Metal      metalgo.Client
	Log        logr.Log
	Namespace  string
	ClusterID  string
	ClusterTag string
}

// Reconcile the Firewall CRD
// +kubebuilder:rbac:groups=metal-stack.io,resources=firewall,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("firewall", req.NamespacedName)
	requeue := ctrl.Result{
		RequeueAfter: time.Second * 10,
	}

	log.Info("running in", "namespace", req.Namespace, "configured for", r.Namespace)
	if req.Namespace != r.Namespace {
		return ctrl.Result{}, nil
	}

	firewallDeployment := &firewallcontrollerv1.FirewallDeployment{}
	if err := r.Seed.Get(ctx, req.NamespacedName, firewallDeployment, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewalldeployments defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}

	err := validate(firewallDeployment)
	if err != nil {
		return requeue, err
	}

	err = r.reconcile(firewallDeployment)
	if err != nil {
		return requeue, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{} // prevents reconcile on status sub resource update
	return ctrl.NewControllerManagedBy(mgr).
		For(&firewallcontrollerv1.FirewallDeployment{}).
		WithEventFilter(pred).
		Complete(r)
}

func validate(firewalldeployment *firewallcontrollerv1.FirewallDeployment) error {
	return nil
}

type firewallReconcileAction string

var (
	// firewallActionRecreate wipe infrastructure status and triggers creation of a new metal firewall
	firewallActionRecreate firewallReconcileAction = "recreate"
	// firewallActionDeleteAndRecreate deletes the firewall, wipe infrastructure status and triggers creation of a new metal firewall
	// occurs when someone changes the firewalltype, firewallimage or additionalnetworks
	// TODO: rename to firewallActionRollingUpdate
	firewallActionDeleteAndRecreate firewallReconcileAction = "delete"
	// firewallActionDoNothing nothing needs to be done for this firewall
	firewallActionDoNothing firewallReconcileAction = "nothing"
	// firewallActionCreate create a new firewall and write infrastructure status
	firewallActionCreate firewallReconcileAction = "create"
	// firewallActionStatusUpdateOnMigrate infrastructure status is not present, but a metal firewall machine is present.
	// this is the case during migration of the shoot to another seed because infrastructure status is not migrated by gardener
	// therefor the status needs to be recreated
	firewallActionStatusUpdateOnMigrate firewallReconcileAction = "migrate"
)

func (r *Reconciler) reconcile(ctx context.Context, firewalldeployment *firewallcontrollerv1.FirewallDeployment) error {
	// detect which next action is required
	action, status, err := r.firewallNextAction(ctx, r)
	if err != nil {
		return err
	}

	switch action {
	case firewallActionDoNothing:
		r.Log.Info("firewall reconciled, nothing to be done", "machine-id", MachineID)
		return nil
	case firewallActionCreate:
		fw, err := r.createFirewall(ctx, firewalldeployment)
		if err != nil {
			return err
		}
		r.Log.Info("firewall created", "machine-id", fw.ID)

		r.providerStatus.Firewall.MachineID = machineID
		return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	case firewallActionRecreate:
		err := deleteFirewallFromStatus(ctx, r)
		if err != nil {
			return err
		}
		r.Log.Info("firewall removed from status", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
		machineID, nodeCIDR, err := createFirewall(ctx, r)
		if err != nil {
			return err
		}
		r.Log.Info("firewall created", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.providerStatus.Firewall.MachineID)

		r.providerStatus.Firewall.MachineID = machineID

		return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	case firewallActionDeleteAndRecreate:
		err := deleteFirewall(ctx, r.machineIDInStatus, r.infrastructureConfig.ProjectID, r.clusterTag, r.mclient)
		if err != nil {
			return err
		}
		r.Log.Info("firewall deleted", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
		err = deleteFirewallFromStatus(ctx, r)
		if err != nil {
			return err
		}
		r.Log.Info("firewall removed from status", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
		machineID, nodeCIDR, err := createFirewall(ctx, r)
		if err != nil {
			return err
		}
		r.Log.Info("firewall created", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.providerStatus.Firewall.MachineID)
		r.providerStatus.Firewall.MachineID = machineID
		return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	case firewallActionStatusUpdateOnMigrate:
		r.providerStatus.Firewall = *status
		return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, r.cluster.Shoot.Spec.Networking.Nodes)
	default:
		return fmt.Errorf("unsupported firewall reconcile action: %s", action)
	}
	return nil
}
func findClusterFirewalls(ctx context.Context, client metalgo.Client, clusterTag, projectID string) ([]*models.V1FirewallResponse, error) {
	resp, err := client.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: projectID,
		Tags:              []string{clusterTag},
	}).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}

	return resp.Payload, nil
}

func (r *Reconciler) firewallNextAction(ctx context.Context, firewallDeployment *v1.FirewallDeployment) (firewallReconcileAction, *metalapi.FirewallStatus, error) {
	resp, err := r.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: firewallDeployment.Spec.ProjectID,
		Tags:              []string{r.ClusterTag},
	}).WithContext(ctx), nil)
	if err != nil {
		return nil, err
	}
	firewalls := resp.Payload
	clusterFirewallAmount := len(firewalls)
	switch clusterFirewallAmount {
	case 0:
		r.Log.Info("firewall does not exist, creating")
		return firewallActionCreate, nil, nil
	case 1:
		fw := firewalls[0]
		if fw.Size.ID != nil && *fw.Size.ID != firewallDeployment.Spec.Size {
			r.Log.Info("firewall size has changed", "machineid", fw.ID, "current", *fw.Size.ID, "new", firewallDeployment.Spec.Size)
			return firewallActionDeleteAndRecreate, nil, nil
		}

		if fw.Allocation != nil && fw.Allocation.Image != nil && fw.Allocation.Image.ID != nil && *fw.Allocation.Image.ID != firewallDeployment.Spec.Image {
			want := firewallDeployment.Spec.Image
			image, err := r.Metal.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(want).WithContext(ctx), nil)
			if err != nil {
				return nil, fmt.Errorf("firewall image not found:%s %w", want, err)
			}

			if image.Payload != nil && image.Payload.ID != nil && *image.Payload.ID != *fw.Allocation.Image.ID {
				r.Log.Info("firewall image has changed", "current", *fw.Allocation.Image.ID, "new", *image.Payload.ID)
				return firewallActionDeleteAndRecreate, nil, nil
			}
		}

		currentNetworks := sets.NewString()
		for _, n := range fw.Allocation.Networks {
			if *n.Networktype == mn.PrivatePrimaryUnshared || *n.Networktype == mn.PrivatePrimaryShared {
				continue
			}
			if *n.Underlay {
				continue
			}
			currentNetworks.Insert(*n.Networkid)
		}
		wantNetworks := sets.NewString()
		for _, n := range firewallDeployment.Spec.Networks {
			wantNetworks.Insert(n)
		}
		if !currentNetworks.Equal(wantNetworks) {
			r.Log.Info("firewall networks have changed", "current", currentNetworks.List(), "new", wantNetworks.List())
			return firewallActionDeleteAndRecreate, nil, nil
		}

		return firewallActionDoNothing, nil, nil
	default:
		err := fmt.Errorf("multiple firewalls exist for this cluster, which is currently unsupported. delete these firewalls and try to keep the one with machine id %q", r.machineIDInStatus)
		r.Log.Error(
			err,
			"multiple firewalls exist for this cluster",
			"clusterID", r.ClusterID,
			"expectedMachineID", firewallDeployment.Status.FirewallIDs,
		)
		return firewallActionDoNothing, nil, err
	}
}

func (r *Reconciler) createFirewall(ctx context.Context, firewallDeployment *v1.FirewallDeployment) (*models.V1FirewallResponse, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := firewallDeployment.Namespace
	name := clusterName + "-firewall-" + uuid.String()[:5]

	// kubeconfig, err := createFirewallControllerKubeconfig(ctx, r)
	// if err != nil {
	// 	return "", "", err
	// }

	// firewallUserData, err := renderFirewallUserData(kubeconfig)
	// if err != nil {
	// 	return "", "", err
	// }

	// assemble firewall allocation request
	var networks []*models.V1MachineAllocationNetwork

	for _, n := range firewallDeployment.Spec.Networks {
		n := n
		network := &models.V1MachineAllocationNetwork{
			Networkid:   &n,
			Autoacquire: pointer.Bool(true),
		}
		networks = append(networks, network)
	}

	createRequest := &models.V1FirewallCreateRequest{
		Description: name + " created by Gardener",
		Name:        name,
		Hostname:    name,
		Sizeid:      &firewallDeployment.Spec.Size,
		Projectid:   &firewallDeployment.Spec.ProjectID,
		Partitionid: &firewallDeployment.Spec.PartitionID,
		Imageid:     &firewallDeployment.Spec.Image,
		SSHPubKeys:  firewallDeployment.Spec.SSHPublicKeys,
		Networks:    networks,
		// UserData:    firewallUserData,
		Tags: []string{r.ClusterTag},
	}

	fcr, err := r.Metal.Firewall().AllocateFirewall(firewall.NewAllocateFirewallParams().WithBody(createRequest).WithContext(ctx), nil)
	if err != nil {
		r.Log.Error(err, "firewall create error")
		return nil, err
	}
	return fcr.Payload, nil
}

// func reconcileEgressIPs(ctx context.Context, r *egressIPReconciler) error {
// 	currentEgressIPs := sets.NewString()

// 	resp, err := r.mclient.IP().FindIPs(ip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
// 		Projectid: r.infrastructureConfig.ProjectID,
// 		Tags:      []string{r.egressTag},
// 		Type:      models.V1IPBaseTypeStatic,
// 	}).WithContext(ctx), nil)
// 	if err != nil {
// 		return &reconciler.RequeueAfterError{
// 			Cause:        fmt.Errorf("failed to list egress ips of cluster %w", err),
// 			RequeueAfter: 30 * time.Second,
// 		}
// 	}

// 	for _, ip := range resp.Payload {
// 		currentEgressIPs.Insert(*ip.Ipaddress)
// 	}

// 	wantEgressIPs := sets.NewString()
// 	for _, egressRule := range r.infrastructureConfig.Firewall.EgressRules {
// 		wantEgressIPs.Insert(egressRule.IPs...)

// 		for _, ip := range egressRule.IPs {
// 			ip := ip
// 			if currentEgressIPs.Has(ip) {
// 				continue
// 			}

// 			resp, err := r.mclient.IP().FindIPs(metalip.NewFindIPsParams().WithBody(&models.V1IPFindRequest{
// 				Ipaddress: ip,
// 				Projectid: r.infrastructureConfig.ProjectID,
// 				Networkid: egressRule.NetworkID,
// 			}).WithContext(ctx), nil)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("error when retrieving ip %s for egress rule %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			switch len(resp.Payload) {
// 			case 0:
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("ip %s for egress rule does not exist", ip),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			case 1:
// 			default:
// 				return fmt.Errorf("ip %s found multiple times", ip)
// 			}

// 			dbIP := resp.Payload[0]
// 			if dbIP.Type != nil && *dbIP.Type != models.V1IPBaseTypeStatic {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("ips for egress rule must be static, but %s is not static", ip),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			if len(dbIP.Tags) > 0 {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("won't use ip %s for egress rules because it does not have an egress tag but it has other tags", *dbIP.Ipaddress),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}

// 			_, err = r.mclient.IP().UpdateIP(metalip.NewUpdateIPParams().WithBody(&models.V1IPUpdateRequest{
// 				Ipaddress: dbIP.Ipaddress,
// 				Tags:      []string{r.egressTag},
// 			}).WithContext(ctx), nil)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("could not tag ip %s for egress usage %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}
// 		}
// 	}

// 	if !currentEgressIPs.Equal(wantEgressIPs) {
// 		toUnTag := currentEgressIPs.Difference(wantEgressIPs)
// 		for _, ip := range toUnTag.List() {
// 			err := clearIPTags(ctx, r.mclient, ip)
// 			if err != nil {
// 				return &reconciler.RequeueAfterError{
// 					Cause:        fmt.Errorf("could not remove egress tag from ip %s %w", ip, err),
// 					RequeueAfter: 30 * time.Second,
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }

// func egressTag(clusterID string) string {
// 	return fmt.Sprintf("%s=%s", tag.ClusterEgress, clusterID)
// }
