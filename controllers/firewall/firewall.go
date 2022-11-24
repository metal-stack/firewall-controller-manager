package firewall

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"

	"github.com/metal-stack/firewall-controller-manager/controllers"
	v1 "github.com/metal-stack/firewall-controller/api/v1"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed       client.Client
	Shoot      client.Client
	Metal      metalgo.Client
	Log        logr.Logger
	Namespace  string
	ClusterID  string
	ClusterTag string
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{} // prevents reconcile on status sub resource update
	return ctrl.NewControllerManagedBy(mgr).
		For(&firewallcontrollerv1.Firewall{}).
		// TODO: find out if we can filter for owner reference
		WithEventFilter(pred).
		Complete(r)
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

	fw := &firewallcontrollerv1.Firewall{}
	if err := r.Seed.Get(ctx, req.NamespacedName, fw, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewall defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}

	err := validate(fw)
	if err != nil {
		return requeue, err
	}

	err = r.reconcile(ctx, fw)
	if err != nil {
		return requeue, err
	}

	return ctrl.Result{}, nil
}

func validate(firewall *firewallcontrollerv1.Firewall) error {
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, fw *firewallcontrollerv1.Firewall) error {
	if !fw.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(fw, controllers.FinalizerName) {
			_, err := r.deleteFirewall(ctx, fw)
			if err != nil {
				return err
			}

			controllerutil.RemoveFinalizer(fw, controllers.FinalizerName)
			if err := r.Seed.Update(ctx, fw); err != nil {
				return err
			}
		}

		return nil
	}

	if !controllerutil.ContainsFinalizer(fw, controllers.FinalizerName) {
		controllerutil.AddFinalizer(fw, controllers.FinalizerName)
		if err := r.Seed.Update(ctx, fw); err != nil {
			return err
		}
	}

	resp, err := r.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		Name:              fw.Spec.Name,
		AllocationProject: fw.Spec.ProjectID,
		Tags:              []string{r.ClusterTag},
	}).WithContext(ctx), nil)
	if err != nil {
		return fmt.Errorf("firewall find error: %w", err)
	}

	switch len(resp.Payload) {
	case 0:
		_, err := r.createFirewall(ctx, fw)
		return err
	case 1:
		current := resp.Payload[0]

		// check whether network prefixes were updated in metal-api
		// prefixes in the firewall machine allocation are just a snapshot when the firewall was created.
		// -> when changing prefixes in the referenced network the firewall does not know about any prefix changes.
		//
		// we replace the prefixes from the snapshot with the actual prefixes that are currently attached to the network.
		// this allows dynamic prefix reconfiguration of the firewall.
		fw.Status.FirewallNetworks = nil
		for _, n := range current.Allocation.Networks {
			n := n
			if n.Networkid == nil {
				continue
			}

			// TODO: network calls could be expensive, maybe add a cache for it
			nwResp, err := r.Metal.Network().FindNetwork(network.NewFindNetworkParams().WithID(*n.Networkid), nil)
			if err != nil {
				return fmt.Errorf("network find error: %w", err)
			}

			fw.Status.FirewallNetworks = append(fw.Status.FirewallNetworks, v1.FirewallNetwork{
				Asn:                 n.Asn,
				Destinationprefixes: n.Destinationprefixes,
				Ips:                 n.Ips,
				Nat:                 n.Nat,
				Networkid:           n.Networkid,
				Networktype:         n.Networktype,
				Prefixes:            nwResp.Payload.Prefixes,
				Vrf:                 n.Vrf,
			})
		}

		machineStatus := v1.MachineStatus{}
		if current.Events != nil && len(current.Events.Log) > 0 {
			log := current.Events.Log[0]

			if log.Event != nil {
				machineStatus.Event = *log.Event
			}
			machineStatus.Message = log.Message
			machineStatus.Time = metav1.NewTime(time.Time(log.Time))
		}
		fw.Status.MachineStatus = machineStatus

		err = r.Seed.Status().Update(ctx, fw)
		if err != nil {
			return fmt.Errorf("error updating status: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("multiple firewalls found")
	}
}

func (r *Reconciler) createFirewall(ctx context.Context, fw *v1.Firewall) (*models.V1FirewallResponse, error) {
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
		Name:        fw.Spec.Name,
		Hostname:    fw.Spec.Name,
		Sizeid:      &fw.Spec.Size,
		Projectid:   &fw.Spec.ProjectID,
		Partitionid: &fw.Spec.PartitionID,
		Imageid:     &fw.Spec.Image,
		SSHPubKeys:  fw.Spec.SSHPublicKeys,
		Networks:    networks,
		UserData:    fw.Spec.Userdata,
		Tags:        []string{r.ClusterTag, controllers.FirewallDeploymentTag(ref.Name)},
	}

	resp, err := r.Metal.Firewall().AllocateFirewall(firewall.NewAllocateFirewallParams().WithBody(createRequest).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall create error: %w", err)
	}

	if resp.Payload == nil || resp.Payload.Allocation == nil || resp.Payload.ID == nil {
		return nil, fmt.Errorf("firewall allocation response is empty")
	}

	fw.Status.FirewallNetworks = nil
	fw.Status.MachineID = *resp.Payload.ID
	for _, n := range resp.Payload.Allocation.Networks {
		n := n
		if n.Networkid == nil {
			continue
		}

		fw.Status.FirewallNetworks = append(fw.Status.FirewallNetworks, v1.FirewallNetwork{
			Asn:                 n.Asn,
			Destinationprefixes: n.Destinationprefixes,
			Ips:                 n.Ips,
			Nat:                 n.Nat,
			Networkid:           n.Networkid,
			Networktype:         n.Networktype,
			Prefixes:            n.Prefixes,
			Vrf:                 n.Vrf,
		})
	}

	err = r.Seed.Status().Update(ctx, fw)
	if err != nil {
		return nil, fmt.Errorf("error updating status: %w", err)
	}

	return resp.Payload, nil
}

func (r *Reconciler) deleteFirewall(ctx context.Context, fw *v1.Firewall) (*models.V1MachineResponse, error) {
	resp, err := r.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(fw.Status.MachineID).WithContext(ctx), nil)
	if err != nil {
		return nil, fmt.Errorf("firewall delete error: %w", err)
	}

	return resp.Payload, nil
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
