package v2

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					ObjectMeta: metav1.ObjectMeta{Name: "connected shortest distance", CreationTimestamp: metav1.NewTime(now.Add(-20 * time.Minute))}, Distance: -1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "middle", CreationTimestamp: metav1.NewTime(now)},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "created"}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallCreated, ConditionTrue, "Created", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
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
					ObjectMeta: metav1.ObjectMeta{Name: "connected older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
			},
			want: []*Firewall{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "highest weight", Annotations: map[string]string{FirewallWeightAnnotation: "100"}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected shortest distance", CreationTimestamp: metav1.NewTime(now.Add(-20 * time.Minute))}, Distance: -1, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected younger", CreationTimestamp: metav1.NewTime(now.Add(5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "connected older", CreationTimestamp: metav1.NewTime(now.Add(-5 * time.Minute))}, Status: FirewallStatus{Conditions: Conditions{NewCondition(FirewallControllerConnected, ConditionTrue, "Connected", ""), NewCondition(FirewallReady, ConditionTrue, "Ready", "")}},
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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			SortFirewallsByImportance(tt.fws)

			if diff := cmp.Diff(tt.want, tt.fws, cmpopts.IgnoreFields(Condition{}, "LastTransitionTime", "LastUpdateTime")); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
