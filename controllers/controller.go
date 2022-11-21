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
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"
)

// Reconciler reconciles a Firewall object
type Reconciler struct {
	Seed      client.Client
	Shoot     client.Client
	Log       logr.Logger
	Namespace string
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
	// first get the metal-api projectID
	firewalls := &firewallcontrollerv1.FirewallList{}
	if err := r.Seed.List(ctx, firewalls, &client.ListOptions{Namespace: req.Namespace}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("no firewalls defined")
			return ctrl.Result{}, nil
		}
		return requeue, err
	}
	err := validate(firewalls)
	if err != nil {
		return requeue, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager boilerplate to setup the Reconciler
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{} // prevents reconcile on status sub resource update
	return ctrl.NewControllerManagedBy(mgr).
		For(&firewallcontrollerv1.Firewall{}).
		WithEventFilter(pred).
		Complete(r)
}

func validate(firewall *firewallcontrollerv1.FirewallList) error {

	return nil
}
