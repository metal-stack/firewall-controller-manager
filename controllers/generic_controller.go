package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	Ctx[O client.Object] struct {
		Ctx    context.Context
		Log    logr.Logger
		Target O
	}
	Reconciler[O client.Object] interface {
		// New returns a new object of O.
		New() O
		// SetStatus sets the status of the reconciled object into the refetched object. this mitigates status updates error due to concurrent modification.
		SetStatus(reconciled O, refetched O)
		Reconcile(rctx *Ctx[O]) error
		Delete(rctx *Ctx[O]) error
	}

	GenericController[O client.Object] struct {
		l          logr.Logger
		namespace  string
		c          client.Client
		reconciler Reconciler[O]
		hasStatus  bool
	}
)

type requeueError struct {
	reason string
	after  time.Duration
}

func (e *requeueError) Error() string {
	return fmt.Sprintf("requeuing after %s: %s", e.after.String(), e.reason)
}

func RequeueAfter(d time.Duration, reason string) error {
	return &requeueError{after: d, reason: reason}
}

type skipStatusUpdateError struct{}

func (e *skipStatusUpdateError) Error() string {
	return fmt.Sprintf("skipping resource status update")
}

func SkipStatusUpdate() error {
	return &skipStatusUpdateError{}
}

func NewGenericController[O client.Object](l logr.Logger, c client.Client, namespace string, reconciler Reconciler[O]) *GenericController[O] {
	return &GenericController[O]{
		l:          l,
		c:          c,
		namespace:  namespace,
		reconciler: reconciler,
		hasStatus:  true,
	}
}

func (g *GenericController[O]) WithoutStatus() *GenericController[O] {
	g.hasStatus = false
	return g
}

func (g *GenericController[O]) logger(req ctrl.Request) logr.Logger {
	return g.l.WithValues("name", req.Name, "namespace", req.Namespace)
}

func (g GenericController[O]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != g.namespace { // should already be filtered out through predicate, but we will check anyway
		return ctrl.Result{}, nil
	}

	var (
		o    = g.reconciler.New()
		log  = g.logger(req)
		rctx = &Ctx[O]{
			Ctx:    ctx,
			Log:    log,
			Target: o,
		}
	)

	if err := g.c.Get(ctx, req.NamespacedName, o, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("resource no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("error retrieving resource: %w", err)
	}

	if !o.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(o, v2.FinalizerName) {
			log.Info("reconciling resource deletion flow")
			err := g.reconciler.Delete(rctx)
			if err != nil {
				var requeueErr *requeueError
				if errors.As(err, &requeueErr) {
					log.Info(requeueErr.Error())
					return ctrl.Result{RequeueAfter: requeueErr.after}, nil //nolint:nilerr we need to return nil such that the requeue works
				}

				log.Error(err, "error during deletion flow")
				return ctrl.Result{}, err
			}

			log.Info("deletion finished, removing finalizer")
			controllerutil.RemoveFinalizer(o, v2.FinalizerName)
			if err := g.c.Update(ctx, o); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(o, v2.FinalizerName) {
		log.Info("adding finalizer")
		controllerutil.AddFinalizer(o, v2.FinalizerName)
		if err := g.c.Update(ctx, o); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to add finalizer: %w", err)
		}
	}

	var (
		skipStatusUpdate = false
		statusErr        error
	)

	if g.hasStatus {
		defer func() {
			if skipStatusUpdate {
				return
			}

			log.Info("updating status")
			refetched := g.reconciler.New()

			statusErr = g.c.Get(ctx, req.NamespacedName, refetched, &client.GetOptions{})
			if statusErr != nil {
				log.Error(statusErr, "unable to fetch resource before status update")
				return
			}

			g.reconciler.SetStatus(o, refetched)

			statusErr = g.c.Status().Update(ctx, refetched)
			if statusErr != nil {
				log.Error(statusErr, "status could not be updated")
			}

			return
		}()
	}

	log.Info("reconciling resource")
	err := g.reconciler.Reconcile(rctx)
	if err != nil {
		var requeueErr *requeueError

		switch {
		case errors.As(err, &requeueErr):
			log.Info(requeueErr.Error())
			return ctrl.Result{RequeueAfter: requeueErr.after}, nil //nolint:nilerr we need to return nil such that the requeue works
		case errors.Is(err, &skipStatusUpdateError{}):
			skipStatusUpdate = true
			return ctrl.Result{}, nil
		default:
			log.Error(err, "error during reconcile")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, statusErr
}

type ItemGetter[O client.ObjectList, E metav1.Object] func(O) []E

func GetOwnedResources[O client.ObjectList, E metav1.Object](ctx context.Context, c client.Client, selector map[string]string, owner metav1.Object, list O, getter ItemGetter[O, E]) (owned []E, orphaned []E, err error) {
	opts := []client.ListOption{client.InNamespace(owner.GetNamespace())}
	if selector != nil {
		opts = append(opts, client.MatchingLabels(selector))
	}

	err = c.List(ctx, list, opts...)
	if err != nil {
		return nil, nil, err
	}

	for _, o := range getter(list) {
		o := o

		if !metav1.IsControlledBy(o, owner) {
			if metav1.GetControllerOf(o) == nil {
				orphaned = append(orphaned, o)
			}

			continue
		}

		owned = append(owned, o)
	}

	return owned, orphaned, nil
}
