package set

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metalgo "github.com/metal-stack/metal-go"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed                  client.Client
	Shoot                 client.Client
	Metal                 metalgo.Client
	Log                   logr.Logger
	Namespace             string
	ClusterID             string
	ClusterTag            string
	ClusterAPIURL         string
	FirewallHealthTimeout time.Duration
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{} // prevents reconcile on status sub resource update
	return ctrl.NewControllerManagedBy(mgr).
		For(&v2.FirewallSet{}). // TODO: trigger a reconcile also for firewall status updates
		WithEventFilter(pred).
		Complete(r)
}

// Reconcile the Firewall CRD
// +kubebuilder:rbac:groups=metal-stack.io,resources=firewall,verbs=get;list;watch;create;update;patch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != r.Namespace {
		return ctrl.Result{}, nil
	}

	set := &v2.FirewallSet{}
	if err := r.Seed.Get(ctx, req.NamespacedName, set, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			r.Log.Info("firewall set resource no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	err := set.Validate()
	if err != nil {
		return ctrl.Result{}, err
	}

	fws := &v2.FirewallList{}
	err = r.Seed.List(ctx, fws, client.InNamespace(r.Namespace))
	if err != nil {
		return ctrl.Result{}, err
	}

	var ownedFirewalls []*v2.Firewall
	for _, fw := range fws.Items {
		fw := fw

		if !metav1.IsControlledBy(&fw, set) {
			continue
		}

		ownedFirewalls = append(ownedFirewalls, &fw)
	}

	defer func() {
		err = r.status(ctx, set, ownedFirewalls)
	}()

	err = r.reconcile(ctx, set, ownedFirewalls)
	if err != nil {
		r.Log.Error(err, "error occurred during reconcile, requeueing")
		return ctrl.Result{ //nolint:nilerr
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) reconcile(ctx context.Context, set *v2.FirewallSet, ownedFirewalls []*v2.Firewall) error {
	r.Log.Info("reconciling firewall set", "name", set.Name, "namespace", set.Namespace, "owned-firewall-count", len(ownedFirewalls))

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

	for _, fw := range ownedFirewalls {
		fw.Spec = set.Spec.Template

		err := r.Seed.Update(ctx, fw, &client.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating firewall spec: %w", err)
		}
	}

	currentAmount := len(ownedFirewalls)

	if currentAmount < set.Spec.Replicas {
		for i := currentAmount; i < set.Spec.Replicas; i++ {
			fw, err := r.createFirewall(ctx, set)
			if err != nil {
				return err
			}
			r.Log.Info("firewall created", "name", fw.Name)
		}
	}

	if currentAmount > set.Spec.Replicas {
		for i := set.Spec.Replicas; i < currentAmount; i++ {
			fw, err := r.deleteFirewallFromSet(ctx, set)
			if err != nil {
				return err
			}
			r.Log.Info("firewall deleted", "name", fw.Name)
		}
	}

	return r.checkOrphans(ctx, set)
}

func (r *Reconciler) checkOrphans(ctx context.Context, set *v2.FirewallSet) error {
	resp, err := r.Metal.Firewall().FindFirewalls(firewall.NewFindFirewallsParams().WithBody(&models.V1FirewallFindRequest{
		AllocationProject: set.Spec.Template.ProjectID,
		Tags:              []string{r.ClusterTag, controllers.FirewallSetTag(set.Name)},
	}).WithContext(ctx), nil)
	if err != nil {
		return err
	}

	if len(resp.Payload) == 0 {
		return nil
	}

	fws := &v2.FirewallList{}
	err = r.Seed.List(ctx, fws, client.InNamespace(r.Namespace))
	if err != nil {
		return err
	}

	var ownedFirewalls []*v2.Firewall
	for _, fw := range fws.Items {
		fw := fw

		if !metav1.IsControlledBy(&fw, set) {
			continue
		}

		ownedFirewalls = append(ownedFirewalls, &fw)
	}

	existingNames := map[string]bool{}
	for _, fw := range ownedFirewalls {
		existingNames[fw.Name] = true
	}

	for _, fw := range resp.Payload {
		if fw.Allocation == nil || fw.Allocation.Name == nil {
			continue
		}
		if _, ok := existingNames[*fw.Allocation.Name]; ok {
			continue
		}

		r.Log.Info("found orphan firewall, deleting orphan", "name", *fw.Allocation.Name, "id", *fw.ID)

		_, err = r.Metal.Machine().FreeMachine(machine.NewFreeMachineParams().WithID(*fw.ID), nil)
		if err != nil {
			return fmt.Errorf("error deleting orphaned firewall: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) deleteFirewallFromSet(ctx context.Context, set *v2.FirewallSet) (*v2.Firewall, error) {
	// TODO: should we prefer deleting some firewalls over others?

	firewalls := v2.FirewallList{}
	err := r.Seed.List(ctx, &firewalls, client.InNamespace(r.Namespace))
	if err != nil {
		return nil, err
	}

	for _, fw := range firewalls.Items {
		fw := fw

		if !metav1.IsControlledBy(&fw, set) {
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

func (r *Reconciler) deleteAllFirewallsFromSet(ctx context.Context, set *v2.FirewallSet) error {
	firewalls := v2.FirewallList{}
	err := r.Seed.List(ctx, &firewalls, client.InNamespace(r.Namespace))
	if err != nil {
		return err
	}

	for _, fw := range firewalls.Items {
		fw := fw

		if !metav1.IsControlledBy(&fw, set) {
			continue
		}

		err = r.Seed.Delete(ctx, &fw, &client.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) createFirewall(ctx context.Context, set *v2.FirewallSet) (*v2.Firewall, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	clusterName := set.Namespace
	name := fmt.Sprintf("%s-firewall-%s", clusterName, uuid.String()[:5])

	fw := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: set.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(set, v2.GroupVersion.WithKind("FirewallSet")),
			},
		},
		Spec: set.Spec.Template,
	}

	// TODO: for backwards-compatibility create firewall object in the shoot cluster as well
	// maybe deploy v1 and create v2 api to manage in the new manner

	err = r.Seed.Create(ctx, fw, &client.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to create firewall resource: %w", err)
	}

	return fw, nil
}

func (r *Reconciler) status(ctx context.Context, set *v2.FirewallSet, fws []*v2.Firewall) error {
	r.Log.Info("updating firewall set status", "name", set.Name, "namespace", set.Namespace)

	status := v2.FirewallSetStatus{}

	for _, fw := range fws {
		fw := fw

		if fw.Status.MachineStatus.Event == "Phoned Home" && !fw.Status.ControllerStatus.Updated.IsZero() {
			status.ReadyReplicas++
		} else if fw.Status.MachineStatus.CrashLoop {
			status.UnhealthyReplicas++
		} else if time.Since(fw.Status.MachineStatus.AllocationTimestamp.Time) > r.FirewallHealthTimeout {
			status.UnhealthyReplicas++
		} else {
			status.ProgressingReplicas++
		}
	}

	set.Status = status

	err := r.Seed.Status().Update(ctx, set)
	if err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	return nil
}
