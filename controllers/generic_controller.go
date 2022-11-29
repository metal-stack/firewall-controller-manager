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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	Reconciler[O client.Object] interface {
		New() O
		Reconcile(ctx context.Context, log logr.Logger, o O) error
		Delete(ctx context.Context, log logr.Logger, o O) error
		Status(ctx context.Context, log logr.Logger, o O) error
	}

	GenericController[O client.Object] struct {
		l          logr.Logger
		namespace  string
		c          client.Client
		reconciler Reconciler[O]
	}
)

var progressingError = fmt.Errorf("reconcile is still progressing")

func StillProgressing() error {
	return progressingError
}

func NewGenericController[O client.Object](l logr.Logger, c client.Client, namespace string, reconciler Reconciler[O]) GenericController[O] {
	return GenericController[O]{
		l:          l,
		c:          c,
		namespace:  namespace,
		reconciler: reconciler,
	}
}

func (g *GenericController[O]) logger(req ctrl.Request) logr.Logger {
	return g.l.WithValues("name", req.Name, "namespace", req.Namespace)
}

func (g GenericController[O]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if req.Namespace != g.namespace { // should already be filtered out through predicate, but we will check anyway
		return ctrl.Result{}, nil
	}

	var (
		o   = g.reconciler.New()
		log = g.logger(req)
	)

	if err := g.c.Get(ctx, req.NamespacedName, o, &client.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("resource no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("error retrieving resource: %w", err)
	}

	if !o.GetDeletionTimestamp().IsZero() {
		if controllerutil.ContainsFinalizer(o, FinalizerName) {
			log.Info("reconciling resource deletion flow")
			err := g.reconciler.Delete(ctx, log, o)
			if err != nil {
				log.Error(err, "error during deletion flow")
				return ctrl.Result{Requeue: true}, nil //nolint:nilerr we need to return nil such that the requeue works
			}

			controllerutil.RemoveFinalizer(o, FinalizerName)
			if err := g.c.Update(ctx, o); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(o, FinalizerName) {
		controllerutil.AddFinalizer(o, FinalizerName)
		if err := g.c.Update(ctx, o); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to add finalizer: %w", err)
		}
	}

	defer func() {
		o := g.reconciler.New()
		if err := g.c.Get(ctx, req.NamespacedName, o, &client.GetOptions{}); err != nil {
			log.Error(fmt.Errorf("error retrieving resource: %w", err), "cannot update status")
			return
		}

		log.Info("updating status")

		err := g.reconciler.Status(ctx, log, o)
		if err != nil {
			log.Error(err, "status could not be reconciled, not updating")
			return
		}

		err = g.c.Status().Update(ctx, o)
		if err != nil {
			log.Error(err, "status could not be updated")
		}
		return
	}()

	log.Info("reconciling resource")
	err := g.reconciler.Reconcile(ctx, log, o)
	if err != nil {
		if errors.Is(err, progressingError) {
			log.Info("still progressing, requeuing...")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil //nolint:nilerr we need to return nil such that the requeue works
		}
		log.Error(err, "error during reconcile, requeueing")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type ItemGetter[O client.ObjectList, E metav1.Object] func(O) []E

func GetOwnedResources[O client.ObjectList, E metav1.Object](ctx context.Context, c client.Client, owner metav1.Object, list O, getter ItemGetter[O, E]) ([]E, error) {
	err := c.List(ctx, list, client.InNamespace(owner.GetNamespace()))
	if err != nil {
		return nil, err
	}

	var owned []E
	for _, o := range getter(list) {
		o := o

		if !metav1.IsControlledBy(o, owner) {
			continue
		}

		owned = append(owned, o)
	}

	return owned, nil
}
