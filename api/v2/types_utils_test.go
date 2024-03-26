package v2

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestConditions(t *testing.T) {
	status := &FirewallStatus{}

	status.Conditions.Set(NewCondition(FirewallCreated, ConditionTrue, "Created", "Firewall was created at 12:00."))
	status.Conditions.Set(NewCondition(FirewallReady, ConditionTrue, "Running", "Firewall is phoning home and alive."))
	status.Conditions.Set(NewCondition(FirewallControllerConnected, ConditionFalse, "NotConnected", "firewall-controller has not yet connected."))

	want := Conditions{
		{
			Type:               FirewallCreated,
			Status:             ConditionTrue,
			LastTransitionTime: status.Conditions.Get(FirewallCreated).LastTransitionTime,
			LastUpdateTime:     status.Conditions.Get(FirewallCreated).LastUpdateTime,
			Reason:             "Created",
			Message:            "Firewall was created at 12:00.",
		},
		{
			Type:               FirewallReady,
			Status:             ConditionTrue,
			LastTransitionTime: status.Conditions.Get(FirewallReady).LastTransitionTime,
			LastUpdateTime:     status.Conditions.Get(FirewallReady).LastUpdateTime,
			Reason:             "Running",
			Message:            "Firewall is phoning home and alive.",
		},
		{
			Type:               FirewallControllerConnected,
			Status:             ConditionFalse,
			LastTransitionTime: status.Conditions.Get(FirewallControllerConnected).LastTransitionTime,
			LastUpdateTime:     status.Conditions.Get(FirewallControllerConnected).LastUpdateTime,
			Reason:             "NotConnected",
			Message:            "firewall-controller has not yet connected.",
		},
	}

	assert.False(t, status.Conditions.Get(FirewallCreated).LastTransitionTime.IsZero())
	assert.False(t, status.Conditions.Get(FirewallCreated).LastUpdateTime.IsZero())

	if diff := cmp.Diff(want, status.Conditions); diff != "" {
		t.Errorf("diff (+got -want):\n %s", diff)
	}

	status.Conditions.Remove(FirewallControllerConnected)

	want = Conditions{
		{
			Type:               FirewallCreated,
			Status:             ConditionTrue,
			LastTransitionTime: status.Conditions.Get(FirewallCreated).LastTransitionTime,
			LastUpdateTime:     status.Conditions.Get(FirewallCreated).LastUpdateTime,
			Reason:             "Created",
			Message:            "Firewall was created at 12:00.",
		},
		{
			Type:               FirewallReady,
			Status:             ConditionTrue,
			LastTransitionTime: status.Conditions.Get(FirewallReady).LastTransitionTime,
			LastUpdateTime:     status.Conditions.Get(FirewallReady).LastUpdateTime,
			Reason:             "Running",
			Message:            "Firewall is phoning home and alive.",
		},
	}

	if diff := cmp.Diff(want, status.Conditions); diff != "" {
		t.Errorf("diff (+got -want):\n %s", diff)
	}
}

func Test_annotationWasRemoved(t *testing.T) {
	tests := []struct {
		name       string
		o          map[string]string
		n          map[string]string
		annotation string
		want       bool
	}{
		{
			name:       "annotation is not present",
			annotation: "c",
			o:          map[string]string{"a": ""},
			n:          map[string]string{"b": ""},
			want:       false,
		},
		{
			name:       "annotation is present",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was added",
			annotation: "c",
			o:          nil,
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was removed",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          nil,
			want:       true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := annotationWasRemoved(event.UpdateEvent{
				ObjectOld: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.o,
					},
				},
				ObjectNew: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.n,
					},
				},
			}, tt.annotation); got != tt.want {
				t.Errorf("annotationWasRemoved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_annotationWasAdded(t *testing.T) {
	tests := []struct {
		name       string
		o          map[string]string
		n          map[string]string
		annotation string
		want       bool
	}{
		{
			name:       "annotation is not present",
			annotation: "c",
			o:          map[string]string{"a": ""},
			n:          map[string]string{"b": ""},
			want:       false,
		},
		{
			name:       "annotation is present",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          map[string]string{"c": ""},
			want:       false,
		},
		{
			name:       "annotation was added",
			annotation: "c",
			o:          nil,
			n:          map[string]string{"c": ""},
			want:       true,
		},
		{
			name:       "annotation was removed",
			annotation: "c",
			o:          map[string]string{"c": ""},
			n:          nil,
			want:       false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := annotationWasAdded(event.UpdateEvent{
				ObjectOld: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.o,
					},
				},
				ObjectNew: &Firewall{
					ObjectMeta: v1.ObjectMeta{
						Annotations: tt.n,
					},
				},
			}, tt.annotation); got != tt.want {
				t.Errorf("annotationWasAdded() = %v, want %v", got, tt.want)
			}
		})
	}
}
