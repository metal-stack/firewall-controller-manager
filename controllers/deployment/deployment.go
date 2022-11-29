package deployment

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/image"

	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/client-go/tools/record"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed          client.Client
	Shoot         client.Client
	Metal         metalgo.Client
	K8sVersion    *semver.Version
	Log           logr.Logger
	Namespace     string
	ClusterID     string
	ClusterTag    string
	ClusterAPIURL string
	Recorder      record.EventRecorder
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallDeployment{}). // TODO: trigger a reconcile also for firewallset status updates
		Named("FirewallDeployment").
		Owns(&v2.FirewallSet{}).
		Complete(r)
}

// Reconcile the Firewall CRD
// +kubebuilder:rbac:groups=metal-stack.io,resources=firewall,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != r.Namespace {
		return ctrl.Result{}, nil
	}

	err := r.ensureFirewallControllerRBAC(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	deploy := &v2.FirewallDeployment{}
	if err := r.Seed.Get(ctx, req.NamespacedName, deploy, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("firewall deployment resource no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	err = deploy.Validate() // TODO: add a validating webhook to perform this validation immediately (https://book.kubebuilder.io/cronjob-tutorial/webhook-implementation.html)
	if err != nil {
		return ctrl.Result{}, err
	}

	sets := &v2.FirewallSetList{}
	err = r.Seed.List(ctx, sets, client.InNamespace(r.Namespace))
	if err != nil {
		return ctrl.Result{}, err
	}

	var ownedSets []*v2.FirewallSet
	for _, set := range sets.Items {
		set := set

		if !metav1.IsControlledBy(&set, deploy) {
			continue
		}

		ownedSets = append(ownedSets, &set)
	}

	defer func() {
		statusErr := r.status(ctx, deploy, ownedSets)
		if statusErr != nil {
			r.Log.Error(statusErr, "error updating status")
		}
	}()

	err = r.reconcile(ctx, deploy, ownedSets)
	if err != nil {
		r.Log.Error(err, "error occurred during reconcile, requeueing")
		return ctrl.Result{ //nolint:nilerr
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, deploy *v2.FirewallDeployment, ownedSets []*v2.FirewallSet) error {
	r.Log.Info("reconciling firewall deployment", "namespace", deploy.Namespace, "owned-set-amount", len(ownedSets))

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
			return fmt.Errorf("unable to add finalizer: %w", err)
		}
	}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if lastSet == nil {
		r.Log.Info("no firewall set is present, creating a new one")

		set, err := r.createFirewallSet(ctx, deploy)
		if err != nil {
			return fmt.Errorf("unable to create firewall set: %w", err)
		}
		r.Recorder.Event(set, "Normal", "Create", fmt.Sprintf("created firewallset %s", set.Name))
		return nil
	}

	oldSets := controllers.Except(ownedSets, lastSet)

	r.Log.Info("firewall sets already exist", "current-set-name", lastSet.Name, "old-set-count", len(oldSets))

	newSetRequired, err := r.isNewSetRequired(ctx, deploy, lastSet)
	if err != nil {
		return err
	}

	if !newSetRequired {
		r.Log.Info("existing firewall set does not need to be rolled, only updating the resource")

		lastUserdata := lastSet.Spec.Template.Userdata

		lastSet.Spec.Replicas = deploy.Spec.Replicas
		lastSet.Spec.Template = deploy.Spec.Template
		lastSet.Spec.Template.Userdata = lastUserdata

		err = r.Seed.Update(ctx, lastSet, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update firewall set: %w", err)
		}
		r.Recorder.Event(lastSet, "Normal", "Update", fmt.Sprintf("updated firewallset %s", lastSet.Name))
	} else {
		// this is recreate strategy: TODO implement rolling update

		r.Log.Info("significant changes detected in the spec, creating new firewall set")
		newSet, err := r.createFirewallSet(ctx, deploy)
		if err != nil {
			return err
		}
		r.Log.Info("created new firewall set", "name", newSet.Name)

		oldSets = append(oldSets, lastSet.DeepCopy())
		r.Recorder.Event(newSet, "Normal", "Recreate", fmt.Sprintf("recreated firewallset old: %s new: %s", lastSet.Name, newSet.Name))
	}

	for _, oldSet := range oldSets {
		r.Log.Info("deleting old firewall set", "name", oldSet.Name)

		oldSet := oldSet
		err = r.Seed.Delete(ctx, oldSet, &client.DeleteOptions{})
		if err != nil {
			return err
		}
		r.Recorder.Event(oldSet, "Normal", "Delete", fmt.Sprintf("deleted firewallset %s", oldSet.Name))
	}

	return nil
}

func (r *Reconciler) status(ctx context.Context, deploy *v2.FirewallDeployment, ownedSets []*v2.FirewallSet) error {
	r.Log.Info("updating firewall deployment status", "name", deploy.Name, "namespace", deploy.Namespace)

	// refetch to prevent updating an old revision
	if err := r.Seed.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, deploy, &client.GetOptions{}); err != nil {
		return err
	}

	status := v2.FirewallDeploymentStatus{}

	lastSet, err := controllers.MaxRevisionOf(ownedSets)
	if err != nil {
		return err
	}

	if lastSet != nil {
		status.ProgressingReplicas = lastSet.Status.ProgressingReplicas
		status.UnhealthyReplicas = lastSet.Status.UnhealthyReplicas
		status.ReadyReplicas = lastSet.Status.ReadyReplicas
	}

	deploy.Status = status

	err = r.Seed.Status().Update(ctx, deploy)
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

func (r *Reconciler) isNewSetRequired(ctx context.Context, deploy *v2.FirewallDeployment, lastSet *v2.FirewallSet) (bool, error) {
	ok := sizeHasChanged(deploy, lastSet)
	if ok {
		r.Log.Info("firewall size has changed", "size", deploy.Spec.Template.Size)
		return ok, nil
	}

	ok, err := osImageHasChanged(ctx, r.Metal, deploy, lastSet)
	if err != nil {
		return false, err
	}
	if ok {
		r.Log.Info("firewall image has changed", "image", deploy.Spec.Template.Image)
		return ok, nil
	}

	ok = networksHaveChanged(deploy, lastSet)
	if ok {
		r.Log.Info("firewall networks have changed", "networks", deploy.Spec.Template.Networks)
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

func (r *Reconciler) createFirewallSet(ctx context.Context, deploy *v2.FirewallDeployment) (*v2.FirewallSet, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	userdata, err := r.createUserdata(ctx)
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
	}

	set.Spec.Template.Userdata = userdata

	err = r.Seed.Create(ctx, set, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall set: %w", err)
	}

	return set, nil
}

func (r *Reconciler) createUserdata(ctx context.Context) (string, error) {
	var (
		ca    []byte
		token string
	)
	if controllers.VersionGreaterOrEqual125(r.K8sVersion) {
		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: r.Namespace,
			},
		}
		err := r.Seed.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return "", err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	} else {
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

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	}

	if token == "" {
		return "", fmt.Errorf("no token was created")
	}

	config := &configv1.Config{
		CurrentContext: r.Namespace,
		Clusters: []configv1.NamedCluster{
			{
				Name: r.Namespace,
				Cluster: configv1.Cluster{
					CertificateAuthorityData: ca,
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
					Token: token,
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
	r.Log.Info("ensuring firewall controller rbac")

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

	if controllers.VersionGreaterOrEqual125(r.K8sVersion) {
		serviceAccountSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: r.Namespace,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, r.Seed, serviceAccountSecret, func() error {
			serviceAccountSecret.Annotations = map[string]string{
				"kubernetes.io/service-account.name": serviceAccount.Name,
			}
			serviceAccountSecret.Type = corev1.SecretTypeServiceAccountToken
			return nil
		})
		if err != nil {
			return fmt.Errorf("error ensuring service account token secret: %w", err)
		}

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
