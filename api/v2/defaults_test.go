package v2

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_firewallDeploymentDefaulter_Default(t *testing.T) {
	tests := []struct {
		name    string
		obj     *FirewallDeployment
		want    *FirewallDeployment
		wantErr error
	}{
		{
			name: "all defaults applied",
			obj: &FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
				Spec: FirewallDeploymentSpec{
					Template: FirewallTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"a": "b",
							},
						},
					},
				},
			},
			want: &FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a",
					Namespace: "b",
				},
				Spec: FirewallDeploymentSpec{
					Replicas: 1,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"a": "b",
						},
					},
					Strategy: StrategyRollingUpdate,
					Template: FirewallTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"a": "b",
							},
						},
						Spec: FirewallSpec{
							Interval: "10s",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := &firewallDeploymentDefaulter{
				log: testr.New(t),
			}

			err := r.Default(context.Background(), tt.obj)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
			if diff := cmp.Diff(tt.want, tt.obj); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
