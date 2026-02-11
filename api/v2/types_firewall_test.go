package v2

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing/synctest"
)

func Test_SortFirewallsByImportance(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		fws  []*Firewall
		want []*Firewall
	}{
		{
			name: "mixed test",
			fws: []*Firewall{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ready older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "oldest", CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute))},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "youngest", CreationTimestamp: metav1.NewTime(now.Add(10 * time.Minute))},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "lowest weight", Annotations: map[string]string{FirewallWeightAnnotation: "-100"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected shortest distance", CreationTimestamp: metav1.NewTime(now.Add(-20 * time.Minute))}, Distance: FirewallShortestDistance, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "middle", CreationTimestamp: metav1.NewTime(now)},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "created"}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallCreated, ConditionTrue, "Created", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Distance: FirewallShortestDistance + 1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ready younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "highest weight", Annotations: map[string]string{FirewallWeightAnnotation: "100"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "no information"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Distance: FirewallShortestDistance + 1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
			},
			want: []*Firewall{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "highest weight", Annotations: map[string]string{FirewallWeightAnnotation: "100"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected shortest distance", CreationTimestamp: metav1.NewTime(now.Add(-20 * time.Minute))}, Distance: FirewallShortestDistance, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Distance: FirewallShortestDistance + 1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Distance: FirewallShortestDistance + 1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ready younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "ready older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "created"}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallCreated, ConditionTrue, "Created", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "youngest", CreationTimestamp: metav1.NewTime(now.Add(10 * time.Minute))},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "middle", CreationTimestamp: metav1.NewTime(now)},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "oldest", CreationTimestamp: metav1.NewTime(now.Add(-10 * time.Minute))},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "no information"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "lowest weight", Annotations: map[string]string{FirewallWeightAnnotation: "-100"}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SortFirewallsByImportance(tt.fws)

			if diff := cmp.Diff(tt.want, tt.fws, cmpopts.IgnoreFields(Condition{}, "LastTransitionTime", "LastUpdateTime")); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}

func Test_EvaluateFirewallStatus(t *testing.T) {
	tests := []struct {
		name          string
		modFn         func(fw *Firewall)
		healthTimeout time.Duration
		createTimeout time.Duration
		want          *FirewallStatusEvalResult
		wantReason    string
	}{
		{
			name:  "ready firewall in running phase",
			modFn: nil,
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusReady,
			},
		},
		{
			name: "unhealthy firewall in running phase due to firewall monitor not reconciling",
			modFn: func(fw *Firewall) {
				fw.Status.Conditions.Set(Condition{
					Type:   FirewallControllerConnected,
					Status: ConditionFalse,
				})
			},
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusUnhealthy,
				Reason: "not all health conditions are true: [Connected]",
			},
		},
		{
			name: "unhealthy firewall in running phase due to firewall not reconciling",
			modFn: func(fw *Firewall) {
				fw.Status.Conditions.Set(Condition{
					Type:   FirewallControllerSeedConnected,
					Status: ConditionFalse,
				})
			},
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusUnhealthy,
				Reason: "not all health conditions are true: [SeedConnected]",
			},
		},
		{
			name: "unhealthy firewall in running phase due to readiness condition false",
			modFn: func(fw *Firewall) {
				fw.Status.Conditions.Set(Condition{
					Type:   FirewallReady,
					Status: ConditionFalse,
				})
			},
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusUnhealthy,
				Reason: "not all health conditions are true: [Ready]",
			},
		},
		{
			name:          "health timeout reached because seed not connected",
			healthTimeout: 5 * time.Minute,
			modFn: func(fw *Firewall) {
				cond := fw.Status.Conditions.Get(FirewallControllerSeedConnected)
				cond.Status = ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusHealthTimeout,
				Reason: "5m0s health timeout exceeded, seed connection lost",
			},
		},
		{
			name:          "health timeout not yet reached",
			healthTimeout: 15 * time.Minute,
			modFn: func(fw *Firewall) {
				cond := fw.Status.Conditions.Get(FirewallControllerSeedConnected)
				cond.Status = ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: &FirewallStatusEvalResult{
				Result:    FirewallStatusUnhealthy,
				Reason:    "not all health conditions are true: [SeedConnected]",
				TimeoutIn: pointer.Pointer(5 * time.Minute),
			},
		},
		{
			name:          "create timeout reached because not provisioned",
			createTimeout: 5 * time.Minute,
			modFn: func(fw *Firewall) {
				fw.Status.Phase = FirewallPhaseCreating
				cond := fw.Status.Conditions.Get(FirewallProvisioned)
				cond.Status = ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: &FirewallStatusEvalResult{
				Result: FirewallStatusCreateTimeout,
				Reason: "5m0s create timeout exceeded, firewall not provisioned in time",
			},
		},
		{
			name:          "create timeout not yet reached",
			createTimeout: 15 * time.Minute,
			modFn: func(fw *Firewall) {
				fw.Status.Phase = FirewallPhaseCreating
				cond := fw.Status.Conditions.Get(FirewallProvisioned)
				cond.Status = ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: &FirewallStatusEvalResult{
				Result:    FirewallStatusProgressing,
				Reason:    "not all health conditions are true: [Provisioned]",
				TimeoutIn: pointer.Pointer(5 * time.Minute),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tenMinutesAgo := time.Now().Add(-10 * time.Minute)

				fw := &Firewall{
					Status: FirewallStatus{
						Phase: FirewallPhaseRunning,
						Conditions: Conditions{
							{
								Type:               FirewallControllerConnected,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallControllerSeedConnected,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallCreated,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallReady,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallProvisioned,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallDistanceConfigured,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
							{
								Type:               FirewallMonitorDeployed,
								Status:             ConditionTrue,
								LastTransitionTime: metav1.NewTime(tenMinutesAgo),
								LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
							},
						},
					},
				}

				if tt.modFn != nil {
					tt.modFn(fw)
				}

				got := EvaluateFirewallStatus(fw, tt.createTimeout, tt.healthTimeout)
				if diff := cmp.Diff(tt.want, got); diff != "" {
					t.Errorf("diff = %s", diff)
				}
			})
		})
	}
}
