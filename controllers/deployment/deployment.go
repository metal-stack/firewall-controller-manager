package deployment

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/metal-stack/firewall-controller-manager/controllers"
	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"
	mn "github.com/metal-stack/metal-lib/pkg/net"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/metal-stack/firewall-controller/api/v1"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed          client.Client
	Shoot         client.Client
	Metal         metalgo.Client
	Log           logr.Logger
	Namespace     string
	ClusterID     string
	ClusterTag    string
	ClusterAPIURL string
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{} // prevents reconcile on status sub resource update
	return ctrl.NewControllerManagedBy(mgr).
		For(&firewallcontrollerv1.FirewallDeployment{}).
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

	err := r.ensureFirewallControllerRBAC(ctx)
	if err != nil {
		return requeue, err
	}

	firewallDeployment := &firewallcontrollerv1.FirewallDeployment{}
	if err := r.Seed.Get(ctx, req.NamespacedName, firewallDeployment, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewalldeployments defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}

	err = validate(firewallDeployment)
	if err != nil {
		return requeue, err
	}

	err = r.reconcile(ctx, firewallDeployment)
	if err != nil {
		return requeue, err
	}

	return ctrl.Result{}, nil
}

func validate(firewalldeployment *firewallcontrollerv1.FirewallDeployment) error {
	return nil
}

func (r *Reconciler) ensureFirewallControllerRBAC(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: r.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Seed, serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			"token-invalidator.resources.gardener.cloud/skip": "true",
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account: %w", err)
	}

	role := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: r.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Seed, role, func() error {
		role.Rules = []rbac.PolicyRule{
			{
				APIGroups: []string{v1.GroupVersion.String()},
				Resources: []string{"firewall"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{v1.GroupVersion.String()},
				Resources: []string{"firewall/status"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role: %w", err)
	}

	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: r.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Seed, roleBinding, func() error {
		roleBinding.RoleRef = rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "Role",
			Name:     "firewall-controller-seed-access",
		}
		roleBinding.Subjects = []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "firewall-controller-seed-access",
				Namespace: r.Namespace,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role binding: %w", err)
	}

	return nil
}

func (r *Reconciler) createUserdata(ctx context.Context) (string, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: r.Namespace,
		},
	}
	err := r.Seed.Get(ctx, client.ObjectKeyFromObject(sa), sa, &client.GetOptions{})
	if err != nil {
		return "", err
	}

	if len(sa.Secrets) == 0 {
		return "", fmt.Errorf("service account %q contains no valid token secret", sa.Name)
	}

	saSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Secrets[0].Name,
			Namespace: r.Namespace,
		},
	}
	err = r.Seed.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
	if err != nil {
		return "", err
	}

	config := &configv1.Config{
		CurrentContext: r.Namespace,
		Clusters: []configv1.NamedCluster{
			{
				Name: r.Namespace,
				Cluster: configv1.Cluster{
					CertificateAuthorityData: saSecret.Data["ca.crt"],
					Server:                   r.ClusterAPIURL,
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: r.Namespace,
				Context: configv1.Context{
					Cluster:  r.Namespace,
					AuthInfo: r.Namespace,
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name: r.Namespace,
				AuthInfo: configv1.AuthInfo{
					Token: string(saSecret.Data["token"]),
				},
			},
		},
	}

	kubeconfig, err := runtime.Encode(configlatest.Codec, config)
	if err != nil {
		return "", fmt.Errorf("unable to encode kubeconfig for firewall: %w", err)
	}

	return renderUserdata(kubeconfig)
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
	action, err := r.firewallNextAction(ctx, firewalldeployment)
	if err != nil {
		return err
	}

	switch action {
	case firewallActionDoNothing:
		r.Log.Info("firewall reconciled, nothing to be done")
		return nil
	case firewallActionCreate:
		userdata, err := r.createUserdata(ctx)
		if err != nil {
			return err
		}

		// TODO: overrides info from the user 🙈
		firewalldeployment.Spec.Template.Spec.Userdata = userdata

		fw, err := r.createFirewall(ctx, firewalldeployment)
		if err != nil {
			return err
		}
		r.Log.Info("firewall created", "name", fw.Name)

		// r.providerStatus.Firewall.MachineID = machineID
		// return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	// case firewallActionRecreate:
	// 	err := deleteFirewallFromStatus(ctx, r)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Log.Info("firewall removed from status", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
	// 	machineID, nodeCIDR, err := createFirewall(ctx, r)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Log.Info("firewall created", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.providerStatus.Firewall.MachineID)

	// 	r.providerStatus.Firewall.MachineID = machineID

	// 	return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	// case firewallActionDeleteAndRecreate:
	// 	err := deleteFirewall(ctx, r.machineIDInStatus, r.infrastructureConfig.ProjectID, r.clusterTag, r.mclient)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Log.Info("firewall deleted", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
	// 	err = deleteFirewallFromStatus(ctx, r)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Log.Info("firewall removed from status", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.machineIDInStatus)
	// 	machineID, nodeCIDR, err := createFirewall(ctx, r)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Log.Info("firewall created", "cluster-id", r.clusterID, "cluster", r.cluster.Shoot.Name, "machine-id", r.providerStatus.Firewall.MachineID)
	// 	r.providerStatus.Firewall.MachineID = machineID
	// 	return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, &nodeCIDR)
	// case firewallActionStatusUpdateOnMigrate:
	// 	r.providerStatus.Firewall = *status
	// 	return updateProviderStatus(ctx, r.c, r.infrastructure, r.providerStatus, r.cluster.Shoot.Spec.Networking.Nodes)
	default:
		return fmt.Errorf("unsupported firewall reconcile action: %s", action)
	}
	return nil
}

// func findClusterFirewalls(ctx context.Context, client metalgo.Client, clusterTag, projectID string) ([]*models.V1FirewallResponse, error) {
// 	resp, err := client.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
// 		AllocationProject: projectID,
// 		Tags:              []string{clusterTag},
// 	}).WithContext(ctx), nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return resp.Payload, nil
// }

func (r *Reconciler) firewallNextAction(ctx context.Context, firewallDeployment *v1.FirewallDeployment) (firewallReconcileAction, error) {
	resp, err := r.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: firewallDeployment.Spec.Template.Spec.ProjectID,
		Tags:              []string{r.ClusterTag, controllers.FirewallDeploymentTag(firewallDeployment.Name)},
	}).WithContext(ctx), nil)
	if err != nil {
		return firewallActionDoNothing, err
	}

	firewalls := resp.Payload
	clusterFirewallAmount := len(firewalls)

	switch clusterFirewallAmount {
	case 0:
		r.Log.Info("firewall does not exist, creating")
		return firewallActionCreate, nil
	case 1:
		fw := firewalls[0]

		ok, err := r.firewallNeedsToBeReplaced(ctx, firewallDeployment, fw)
		if err != nil {
			return firewallActionDoNothing, err
		}

		if ok {
			return firewallActionDeleteAndRecreate, nil
		}

		return firewallActionDoNothing, nil
	default:
		err := fmt.Errorf("multiple firewalls exist for this cluster, which is currently unsupported. cleanup manually created firewalls.")
		r.Log.Error(
			err,
			"multiple firewalls exist for this cluster",
			"clusterID", r.ClusterID,
			"expectedMachineID", firewallDeployment.Status.FirewallIDs,
		)

		return firewallActionDoNothing, nil
	}
}

func (r *Reconciler) firewallNeedsToBeReplaced(ctx context.Context, fwd *v1.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
	ok, err := sizeHasChanged(fwd, fw)
	if err != nil {
		return false, err
	}
	if ok {
		r.Log.Info("firewall size has changed", "size", fwd.Spec.Template.Spec.Size)
		return ok, nil
	}

	ok, err = osImageHasChanged(ctx, r.Metal, fwd, fw)
	if err != nil {
		return false, err
	}
	if ok {
		r.Log.Info("firewall image has changed", "image", fwd.Spec.Template.Spec.Image)
		return ok, nil
	}

	ok = networksHaveChanged(fwd, fw)
	if ok {
		r.Log.Info("firewall networks have changed", "networks", fwd.Spec.Template.Spec.Networks)
		return ok, nil
	}

	return false, nil
}

func sizeHasChanged(fwd *v1.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
	if fw.ID == nil {
		return false, fmt.Errorf("firewall id is nil")
	}
	if fw.Size == nil || fw.Size.ID == nil {
		return false, fmt.Errorf("firewall size is nil")
	}

	return *fw.Size.ID != fwd.Spec.Template.Spec.Size, nil
}

func osImageHasChanged(ctx context.Context, m metalgo.Client, fwd *v1.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
	if fw.Allocation == nil {
		return false, fmt.Errorf("firewall allocation is nil")
	}
	if fw.Allocation.Image == nil || fw.Allocation.Image.ID == nil {
		return false, fmt.Errorf("firewall image is nil")
	}

	if *fw.Allocation.Image.ID != fwd.Spec.Template.Spec.Image {
		want := fwd.Spec.Template.Spec.Image
		image, err := m.Image().FindLatestImage(image.NewFindLatestImageParams().WithID(want).WithContext(ctx), nil)
		if err != nil {
			return false, fmt.Errorf("latest firewall image not found:%s %w", want, err)
		}

		if image.Payload != nil && image.Payload.ID != nil && *image.Payload.ID != *fw.Allocation.Image.ID {
			return true, nil
		}
	}

	return false, nil
}

func networksHaveChanged(fwd *v1.FirewallDeployment, fw *models.V1FirewallResponse) bool {
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
	for _, n := range fwd.Spec.Template.Spec.Networks {
		wantNetworks.Insert(n)
	}

	return !currentNetworks.Equal(wantNetworks)
}

func (r *Reconciler) createFirewall(ctx context.Context, firewallDeployment *v1.FirewallDeployment) (*v1.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := firewallDeployment.Namespace
	name := clusterName + "-firewall-" + uuid.String()[:5]

	fw := &v1.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: firewallDeployment.Namespace,
			// TODO: Do we need to set OwnerReferences by ourselves?
		},
		Spec: firewallDeployment.Spec.Template.Spec,
	}

	// TODO: for backwards-compatibility create firewall object in the shoot cluster as well
	// maybe deploy v1 and create v2 api to manage in the new manner

	err = r.Seed.Create(ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}
