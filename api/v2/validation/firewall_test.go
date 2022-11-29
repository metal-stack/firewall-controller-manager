package validation

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
)

func Test_firewallValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		f       *v2.Firewall
		wantErr error
	}{
		{
			name: "valid",
			f: &v2.Firewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "firewall-123",
					Namespace: "default",
				},
				Spec: v2.FirewallSpec{
					Interval:          "10s",
					ControllerURL:     "https://metal-stack.io/controller.img",
					ControllerVersion: "v",
					Image:             "image-a",
					PartitionID:       "partition-a",
					ProjectID:         "project-a",
					Size:              "size-a",
					Networks:          []string{"internet"},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallValidator()

			got := v.ValidateCreate(context.TODO(), tt.f)
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}
