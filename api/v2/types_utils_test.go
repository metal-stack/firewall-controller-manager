package v2

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
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
