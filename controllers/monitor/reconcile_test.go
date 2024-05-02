package monitor

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_significantFirewallStatusChange(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		o    v2.FirewallStatus
		n    v2.FirewallStatus
		want bool
	}{
		{
			name: "controller registers",
			o: v2.FirewallStatus{
				ControllerStatus: nil,
			},
			n: v2.FirewallStatus{
				ControllerStatus: &v2.ControllerConnection{
					ActualVersion: "1.2.3",
					Updated:       v1.NewTime(now),
				},
			},
			want: true,
		},
		{
			name: "conditions not really changed",
			o: v2.FirewallStatus{
				Conditions: v2.Conditions{
					v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", now.Add(-1*time.Minute))),
				},
			},
			n: v2.FirewallStatus{
				Conditions: v2.Conditions{
					v2.NewCondition(v2.FirewallControllerSeedConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled firewall at %s.", now.Add(1*time.Minute))),
				},
			},
			want: false,
		},
		{
			name: "controller reconnects",
			o: v2.FirewallStatus{
				Conditions: v2.Conditions{
					v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionFalse, "StoppedReconciling", fmt.Sprintf("Controller has stopped reconciling since %s to shoot.", now)),
				},
			},
			n: v2.FirewallStatus{
				Conditions: v2.Conditions{
					v2.NewCondition(v2.FirewallControllerConnected, v2.ConditionTrue, "Connected", fmt.Sprintf("Controller reconciled shoot at %s.", now)),
				},
			},
			want: true,
		},
		{
			name: "controller version update",
			o: v2.FirewallStatus{
				ControllerStatus: &v2.ControllerConnection{
					ActualVersion: "v1.2.3",
				},
			},
			n: v2.FirewallStatus{
				ControllerStatus: &v2.ControllerConnection{
					ActualVersion: "v1.2.4",
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := significantFirewallStatusChange(tt.o, tt.n)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
