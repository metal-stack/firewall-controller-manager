package v2

import (
	"context"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// ReconcileAnnotation can be used to trigger a reconciliation of a resource managed by a controller.
	// The value of the annotation does not matter, the controller will cleanup the annotation automatically and trigger a reconciliation of the resource.
	ReconcileAnnotation = "firewall.metal-stack.io/reconcile"
	// MaintenanceAnnotation can be used to trigger a maintenance reconciliation for which a controller might have special behavior.
	// The value of the annotation does not matter, the controller will cleanup the annotation automatically.
	MaintenanceAnnotation = "firewall.metal-stack.io/maintain"
	// RollSetAnnotation can be used to trigger a rolling update of a firewall deployment.
	// The value of the annotation needs to be true otherwise the controller will ignore it.
	RollSetAnnotation = "firewall.metal-stack.io/roll-set"
	// RevisionAnnotation stores the revision number of a resource.
	RevisionAnnotation = "firewall.metal-stack.io/revision"

	// FirewallNoControllerConnectionAnnotation can be used as an annotation to the firewall resource in order
	// to indicate that the firewall-controller does not connect to the firewall monitor. this way, the replica
	// set will become healthy without a controller connection.
	//
	// this can be useful to silence a problem temporarily and was used in the past for migration of firewall-controller v1.
	FirewallNoControllerConnectionAnnotation = "firewall.metal-stack.io/no-controller-connection"
	// FirewallControllerManagedByAnnotation is used as tag for creating a firewall to indicate who is managing the firewall.
	FirewallControllerManagedByAnnotation = "firewall.metal-stack.io/managed-by"
	// FirewallWeightAnnotation is considered when deciding which firewall is thrown away on scale down.
	// Value must be parsable as an integer. Firewalls with higher weight are kept longer.
	// Defaults to 0 if no annotation is present. Negative values are allowed.
	FirewallWeightAnnotation = "firewall.metal-stack.io/weight"

	// FirewallControllerSetAnnotation is a tag added to the firewall entity indicating to which set a firewall belongs to.
	FirewallControllerSetAnnotation = "firewall.metal.stack.io/set"
)

// IsAnnotationPresent returns true if the given object has an annotation with a given
// key, the value of this annotation does not matter.
func IsAnnotationPresent(o client.Object, key string) bool {
	_, ok := o.GetAnnotations()[key]
	return ok
}

// IsAnnotationTrue returns true if the given object has an annotation with a given
// key and the value of this annotation is a true boolean.
func IsAnnotationTrue(o client.Object, key string) bool {
	enabled, err := strconv.ParseBool(o.GetAnnotations()[key])
	return err == nil && enabled
}

// IsAnnotationFalse returns true if the given object has an annotation with a given
// key and the value of this annotation is a false boolean.
func IsAnnotationFalse(o client.Object, key string) bool {
	enabled, err := strconv.ParseBool(o.GetAnnotations()[key])
	return err == nil && !enabled
}

// RemoveAnnotation removes an annotation by a given key from an object if present by updating it with the given client.
func AddAnnotation(ctx context.Context, c client.Client, o client.Object, key, value string) error {
	annotations := o.GetAnnotations()

	if annotations == nil {
		annotations = map[string]string{}
	}

	if existingValue, ok := annotations[key]; ok && existingValue == value {
		return nil
	}

	annotations[key] = value

	o.SetAnnotations(annotations)

	err := c.Update(ctx, o)
	if err != nil {
		return err
	}

	return nil
}

// RemoveAnnotation removes an annotation by a given key from an object if present by updating it with the given client.
func RemoveAnnotation(ctx context.Context, c client.Client, o client.Object, key string) error {
	annotations := o.GetAnnotations()

	if annotations == nil {
		return nil
	}

	_, ok := annotations[key]
	if !ok {
		return nil
	}

	delete(annotations, key)

	o.SetAnnotations(annotations)

	err := c.Update(ctx, o)
	if err != nil {
		return err
	}

	return nil
}

func annotationWasRemoved(update event.UpdateEvent, annotation string) bool {
	if cmp.Equal(update.ObjectOld.GetAnnotations(), update.ObjectNew.GetAnnotations()) {
		return false
	}

	var (
		_, o = update.ObjectOld.GetAnnotations()[annotation]
		_, n = update.ObjectNew.GetAnnotations()[annotation]
	)

	if n {
		return false
	}

	if !o {
		return false
	}

	return o && !n
}

func annotationWasAdded(update event.UpdateEvent, annotation string) bool {
	if cmp.Equal(update.ObjectOld.GetAnnotations(), update.ObjectNew.GetAnnotations()) {
		return false
	}

	var (
		_, o = update.ObjectOld.GetAnnotations()[annotation]
		_, n = update.ObjectNew.GetAnnotations()[annotation]
	)

	if o {
		return false
	}

	if !n {
		return false
	}

	return !o && n
}

// AnnotationAddedPredicate returns a predicate when the given annotation key was added.
func AnnotationAddedPredicate(annotation string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(update event.UpdateEvent) bool {
			return annotationWasAdded(update, annotation)
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			return false
		},
	}
}

// AnnotationRemovedPredicate returns a predicate when the given annotation key was removed.
func AnnotationRemovedPredicate(annotation string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(update event.UpdateEvent) bool {
			return annotationWasRemoved(update, annotation)
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			return false
		},
	}
}
