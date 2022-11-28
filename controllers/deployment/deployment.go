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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	mn "github.com/metal-stack/metal-lib/pkg/net"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/models"

	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
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
		For(&v2.FirewallDeployment{}). // TODO: trigger a reconcile also for firewallset status updates
		WithEventFilter(pred).
		Complete(r)
}

// Reconcile the Firewall CRD
// +kubebuilder:rbac:groups=metal-stack.io,resources=firewall,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("firewalldeployment", req.NamespacedName)
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

	deploy := &v2.FirewallDeployment{}
	if err := r.Seed.Get(ctx, req.NamespacedName, deploy, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewalldeployment defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}

	err = deploy.Validate() // TODO: add a validating webhook to perform this validation immediately (https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
	if err != nil {
		return requeue, err
	}

	sets := &v2.FirewallSetList{}
	err = r.Seed.List(ctx, sets, client.InNamespace(r.Namespace))
	if err != nil {
		return requeue, err
	}

	var ownedSets []*v2.FirewallSet
	for _, s := range sets.Items {
		s := s

		if !metav1.IsControlledBy(&s, deploy) {
			continue
		}

		ownedSets = append(ownedSets, &s)
	}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return requeue, err
	}

	defer func() {
		err = r.status(ctx, deploy, lastSet)
	}()

	err = r.reconcile(ctx, deploy, ownedSets)
	if err != nil {
		r.Log.Error(err, "error occurred during reconcile")
		return requeue, err
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, deploy *v2.FirewallDeployment, ownedSets []*v2.FirewallSet) error {
	if !deploy.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(deploy, controllers.FinalizerName) {
			err := r.deleteAllSets(ctx, deploy)
			if err != nil {
				return err
			}

			controllerutil.RemoveFinalizer(deploy, controllers.FinalizerName)
			if err := r.Seed.Update(ctx, deploy); err != nil {
				return err
			}
		}

		return nil
	}

	if !controllerutil.ContainsFinalizer(deploy, controllers.FinalizerName) {
		controllerutil.AddFinalizer(deploy, controllers.FinalizerName)
		if err := r.Seed.Update(ctx, deploy); err != nil {
			return err
		}
	}

	// FIXME: implement
	// goal: always have one set doing it's job properly (most recent), try to scale down / delete the rest according to strategy
	if len(ownedSets) == 0 {
		_, err := r.createFirewallSet(ctx, deploy)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) status(ctx context.Context, deploy *v2.FirewallDeployment, set *v2.FirewallSet) error {
	status := v2.FirewallDeploymentStatus{}

	status.ProgressingReplicas = set.Status.ProgressingReplicas
	status.UnhealthyReplicas = set.Status.UnhealthyReplicas
	status.ReadyReplicas = set.Status.ReadyReplicas

	deploy.Status = status

	err := r.Seed.Status().Update(ctx, deploy)
	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	return nil
}

func (r *Reconciler) deleteAllSets(ctx context.Context, deploy *v2.FirewallDeployment) error {
	sets := v2.FirewallSetList{}
	err := r.Seed.List(ctx, &sets, client.InNamespace(r.Namespace))
	if err != nil {
		return err
	}

	for _, s := range sets.Items {
		s := s

		if !metav1.IsControlledBy(&s, deploy) {
			continue
		}

		err = r.Seed.Delete(ctx, &s, &client.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) isNewSetRequired(ctx context.Context, fwd *v2.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
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

func sizeHasChanged(fwd *v2.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
	if fw.ID == nil {
		return false, fmt.Errorf("firewall id is nil")
	}
	if fw.Size == nil || fw.Size.ID == nil {
		return false, fmt.Errorf("firewall size is nil")
	}

	return *fw.Size.ID != fwd.Spec.Template.Spec.Size, nil
}

func osImageHasChanged(ctx context.Context, m metalgo.Client, fwd *v2.FirewallDeployment, fw *models.V1FirewallResponse) (bool, error) {
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

func networksHaveChanged(fwd *v2.FirewallDeployment, fw *models.V1FirewallResponse) bool {
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

func (r *Reconciler) createFirewallSet(ctx context.Context, deploy *v2.FirewallDeployment) (*v2.FirewallSet, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	userdata, err := r.createUserdata(ctx)
	if err != nil {
		return nil, err
	}

	name := deploy.Name + uuid.String()[:5]

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
	}

	// TODO: overrides info from the user 🙈
	set.Spec.Template.Spec.Userdata = userdata

	err = r.Seed.Create(ctx, set, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return set, nil
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
				APIGroups: []string{v2.GroupVersion.String()},
				Resources: []string{"firewall"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{v2.GroupVersion.String()},
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
