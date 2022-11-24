package set

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	corev1 "k8s.io/api/core/v1"

	"github.com/metal-stack/firewall-controller-manager/controllers"
	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/metal-stack/firewall-controller/api/v1"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-go/api/client/firewall"
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
		For(&firewallcontrollerv1.FirewallSet{}).
		WithEventFilter(pred).
		Complete(r)
}

// Reconcile the Firewall CRD
// +kubebuilder:rbac:groups=metal-stack.io,resources=firewall,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("firewallset", req.NamespacedName)
	requeue := ctrl.Result{
		RequeueAfter: time.Second * 10,
	}

	log.Info("running in", "namespace", req.Namespace, "configured for", r.Namespace)
	if req.Namespace != r.Namespace {
		return ctrl.Result{}, nil
	}

	firewallSet := &firewallcontrollerv1.FirewallSet{}
	if err := r.Seed.Get(ctx, req.NamespacedName, firewallSet, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewallset defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}

	err := validate(firewallSet)
	if err != nil {
		return requeue, err
	}

	err = r.reconcile(ctx, firewallSet)
	if err != nil {
		return requeue, err
	}

	return ctrl.Result{}, nil
}

func validate(firewallset *firewallcontrollerv1.FirewallSet) error {
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, set *firewallcontrollerv1.FirewallSet) error {
	if !set.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(set, controllers.FinalizerName) {
			err := r.deleteAllFirewallsFromSet(ctx, set)
			if err != nil {
				return err
			}

			controllerutil.RemoveFinalizer(set, controllers.FinalizerName)
			if err := r.Seed.Update(ctx, set); err != nil {
				return err
			}
		}

		return nil
	}

	if !controllerutil.ContainsFinalizer(set, controllers.FinalizerName) {
		controllerutil.AddFinalizer(set, controllers.FinalizerName)
		if err := r.Seed.Update(ctx, set); err != nil {
			return err
		}
	}

	resp, err := r.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: set.Spec.Template.Spec.ProjectID,
		Tags:              []string{r.ClusterTag, controllers.FirewallSetTag(set.Name)},
	}).WithContext(ctx), nil)
	if err != nil {
		return err
	}

	currentAmount := len(resp.Payload)

	if currentAmount == set.Spec.Replicas {
		return nil
	}

	if currentAmount < set.Spec.Replicas {
		userdata, err := r.createUserdata(ctx)
		if err != nil {
			return err
		}

		// TODO: overrides info from the user 🙈
		set.Spec.Template.Spec.Userdata = userdata

		for i := currentAmount; i < set.Spec.Replicas; i++ {
			fw, err := r.createFirewall(ctx, set)
			if err != nil {
				return err
			}
			r.Log.Info("firewall created", "name", fw.Name)
		}

		return nil
	}

	if currentAmount > set.Spec.Replicas {
		for i := set.Spec.Replicas; i < currentAmount; i++ {
			fw, err := r.deleteFirewallFromSet(ctx, set)
			if err != nil {
				return err
			}
			r.Log.Info("firewall deleted", "name", fw.Name)
		}

		return nil
	}

	return nil
}

func (r *Reconciler) deleteFirewallFromSet(ctx context.Context, set *v1.FirewallSet) (*v1.Firewall, error) {
	// TODO: should we prefer deleting some firewalls over others?

	firewalls := v1.FirewallList{}
	err := r.Seed.List(ctx, &firewalls, client.InNamespace(r.Namespace))
	if err != nil {
		return nil, err
	}

	for _, fw := range firewalls.Items {
		fw := fw

		ref := metav1.GetControllerOf(&fw)
		if ref == nil || ref.Name != set.Name {
			continue
		}

		err = r.Seed.Delete(ctx, &fw, &client.DeleteOptions{})
		if err != nil {
			return nil, err
		}

		return &fw, nil
	}

	return nil, fmt.Errorf("no firewall found for deletion")
}

func (r *Reconciler) deleteAllFirewallsFromSet(ctx context.Context, set *v1.FirewallSet) error {
	firewalls := v1.FirewallList{}
	err := r.Seed.List(ctx, &firewalls, client.InNamespace(r.Namespace))
	if err != nil {
		return err
	}

	for _, fw := range firewalls.Items {
		fw := fw

		ref := metav1.GetControllerOf(&fw)
		if ref == nil || ref.Name != set.Name {
			continue
		}

		err = r.Seed.Delete(ctx, &fw, &client.DeleteOptions{})
		if err != nil {
			return err
		}
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

func (r *Reconciler) createFirewall(ctx context.Context, set *v1.FirewallSet) (*v1.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := set.Namespace
	name := clusterName + "-firewall-" + uuid.String()[:5]

	fw := &v1.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: set.Namespace,
			// TODO: Do we need to set OwnerReferences by ourselves?
		},
		Spec: set.Spec.Template.Spec,
	}

	// TODO: for backwards-compatibility create firewall object in the shoot cluster as well
	// maybe deploy v1 and create v2 api to manage in the new manner

	err = r.Seed.Create(ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}
