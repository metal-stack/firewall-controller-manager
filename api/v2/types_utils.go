package v2

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	RollSetAnnotation  = "firewall.metal-stack.io/roll-set"
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
