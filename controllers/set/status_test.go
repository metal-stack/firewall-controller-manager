package set

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_controller_evaluateFirewallConditions(t *testing.T) {
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)

	tests := []struct {
		name          string
		modFn         func(fw *v2.Firewall)
		healthTimeout time.Duration
		createTimeout time.Duration
		want          status
	}{
		{
			name:  "ready firewall in running phase",
			modFn: nil,
			want:  statusReady,
		},
		{
			name: "unhealthy firewall in running phase due to firewall monitor not reconciling",
			modFn: func(fw *v2.Firewall) {
				fw.Status.Conditions.Set(v2.Condition{
					Type:   v2.FirewallControllerConnected,
					Status: v2.ConditionFalse,
				})
			},
			want: statusUnhealthy,
		},
		{
			name: "unhealthy firewall in running phase due to firewall not reconciling",
			modFn: func(fw *v2.Firewall) {
				fw.Status.Conditions.Set(v2.Condition{
					Type:   v2.FirewallControllerSeedConnected,
					Status: v2.ConditionFalse,
				})
			},
			want: statusUnhealthy,
		},
		{
			name: "unhealthy firewall in running phase due to readiness condition false",
			modFn: func(fw *v2.Firewall) {
				fw.Status.Conditions.Set(v2.Condition{
					Type:   v2.FirewallReady,
					Status: v2.ConditionFalse,
				})
			},
			want: statusUnhealthy,
		},
		{
			name: "unhealthy firewall in running phase due to readiness condition false",
			modFn: func(fw *v2.Firewall) {
				fw.Status.Conditions.Set(v2.Condition{
					Type:   v2.FirewallReady,
					Status: v2.ConditionFalse,
				})
			},
			want: statusUnhealthy,
		},
		{
			name:          "health timeout reached because seed not connected",
			healthTimeout: 5 * time.Minute,
			modFn: func(fw *v2.Firewall) {
				cond := fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)
				cond.Status = v2.ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: statusHealthTimeout,
		},
		{
			name:          "health timeout not yet reached",
			healthTimeout: 15 * time.Minute,
			modFn: func(fw *v2.Firewall) {
				cond := fw.Status.Conditions.Get(v2.FirewallControllerSeedConnected)
				cond.Status = v2.ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: statusUnhealthy,
		},
		{
			name:          "create timeout reached because not provisioned",
			createTimeout: 5 * time.Minute,
			modFn: func(fw *v2.Firewall) {
				fw.Status.Phase = v2.FirewallPhaseCreating
				cond := fw.Status.Conditions.Get(v2.FirewallProvisioned)
				cond.Status = v2.ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: statusCreateTimeout,
		},
		{
			name:          "create timeout not yet reached",
			createTimeout: 15 * time.Minute,
			modFn: func(fw *v2.Firewall) {
				fw.Status.Phase = v2.FirewallPhaseCreating
				cond := fw.Status.Conditions.Get(v2.FirewallProvisioned)
				cond.Status = v2.ConditionFalse
				fw.Status.Conditions.Set(*cond)
			},
			want: statusProgressing,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.New(&config.NewControllerConfig{
				FirewallHealthTimeout: tt.healthTimeout,
				CreateTimeout:         tt.createTimeout,
				SkipValidation:        true,
			})
			require.NoError(t, err)

			c := controller{
				c: cfg,
			}

			fw := &v2.Firewall{
				Status: v2.FirewallStatus{
					Phase: v2.FirewallPhaseRunning,
					Conditions: v2.Conditions{
						{
							Type:               v2.FirewallControllerConnected,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallControllerSeedConnected,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallCreated,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallReady,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallProvisioned,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallDistanceConfigured,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
						{
							Type:               v2.FirewallMonitorDeployed,
							Status:             v2.ConditionTrue,
							LastTransitionTime: metav1.NewTime(tenMinutesAgo),
							LastUpdateTime:     metav1.NewTime(tenMinutesAgo),
						},
					},
				},
			}

			if tt.modFn != nil {
				tt.modFn(fw)
			}

			got := c.evaluateFirewallConditions(fw)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff = %s", diff)
			}
		})
	}
}
