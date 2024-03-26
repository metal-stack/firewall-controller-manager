package v2

import (
	"context"
	"strconv"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// FinalizerName is the finalizer name used by this controller.
	FinalizerName = "firewall.metal-stack.io/firewall-controller-manager"
	// ReconcileAnnotation can be used to trigger a reconciliation of a resource managed by a controller.
	// The value of the annotation does not matter, the controller will cleanup the annotation automatically and trigger a reconciliation of the resource.
	ReconcileAnnotation = "firewall.metal-stack.io/reconcile"
	// RollSetAnnotation can be used to trigger a rolling update of a firewall deployment.
	RollSetAnnotation = "firewall.metal-stack.io/roll-set"
	// RevisionAnnotation stores the revision number of a resource.
	RevisionAnnotation = "firewall.metal-stack.io/revision"
)

// ConditionStatus is the status of a condition.
type ConditionStatus string

// ConditionType is a string alias.
type ConditionType string

// Condition holds the information about the state of a resource.
type Condition struct {
	// Type of the condition.
	Type ConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Last time the condition was updated.
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	// The reason for the condition's last transition.
	Reason string `json:"reason"`
	// A human readable message indicating details about the transition.
	Message string `json:"message"`
}

const (
	// ConditionTrue means a resource is in the condition.
	ConditionTrue ConditionStatus = "True"
	// ConditionFalse means a resource is not in the condition.
	ConditionFalse ConditionStatus = "False"
	// ConditionUnknown means Gardener can't decide if a resource is in the condition or not.
	ConditionUnknown ConditionStatus = "Unknown"
)

type Conditions []Condition

// NewCondition creates a new condition.
func NewCondition(t ConditionType, status ConditionStatus, reason, message string) Condition {
	return Condition{
		Type:               t,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetCondition returns the condition with the provided type.
func (cs Conditions) Get(t ConditionType) *Condition {
	for i := range cs {
		c := cs[i]
		if c.Type == t {
			return &c
		}
	}
	return nil
}

// SetCondition updates the conditions to include the provided condition. If the condition that
// we are about to add already exists and has the same status, reason and message then we are not going to update.
func (cs *Conditions) Set(condition Condition) {
	currentCond := cs.Get(condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason && currentCond.Message == condition.Message {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	newConditions := cs.filterOutCondition(condition.Type)
	*cs = append(newConditions, condition)
}

// RemoveCondition removes the condition with the provided type.
func (cs *Conditions) Remove(t ConditionType) {
	*cs = cs.filterOutCondition(t)
}

// filterOutCondition returns a new slice of conditions without conditions with the provided type.
func (cs Conditions) filterOutCondition(t ConditionType) Conditions {
	var newConditions Conditions
	for _, c := range cs {
		if c.Type == t {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// RemoveAnnotation removes an annotation by a given key from an object if present by updating it with the given client.
// It returns true when the annotation was present and removed and an error if the update process went wrong.
func RemoveAnnotation(ctx context.Context, c client.Client, o client.Object, key string) (bool, error) {
	annotations := o.GetAnnotations()

	_, ok := annotations[key]
	if !ok {
		return false, nil
	}

	delete(annotations, key)

	o.SetAnnotations(annotations)

	err := c.Update(ctx, o)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SkipAnnotationRemoval returns a predicate when the given annotation key was cleaned up.
func SkipAnnotationRemoval(annotation string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(update event.UpdateEvent) bool {
			return !annotationWasRemoved(update, annotation)
		},
	}
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

// IsAnnotationTrue returns true if the given object has an annotation with a given
// key and the value of this annotation is a true boolean.
func IsAnnotationTrue(o client.Object, key string) bool {
	enabled, err := strconv.ParseBool(o.GetAnnotations()[key])
	return err == nil && enabled
}
