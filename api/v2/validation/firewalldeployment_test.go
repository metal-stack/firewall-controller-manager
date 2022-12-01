package validation

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_firewallDeploymentValidator_ValidateCreate(t *testing.T) {
	valid := &v2.FirewallDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall",
			Namespace: "default",
		},
		Spec: v2.FirewallDeploymentSpec{
			Strategy: v2.StrategyRollingUpdate,
			Template: v2.FirewallSpec{
				Interval:          "10s",
				ControllerURL:     "https://metal-stack.io/controller.img",
				ControllerVersion: "v",
				Image:             "image-a",
				Partition:         "partition-a",
				Project:           "project-a",
				Size:              "size-a",
				Networks:          []string{"internet"},
				EgressRules: []v2.EgressRuleSNAT{
					{
						NetworkID: "network-a",
						IPs:       []string{"1.2.3.4"},
					},
				},
				InternalPrefixes: []string{"1.2.3.0/24"},
				RateLimits: []v2.RateLimit{
					{
						NetworkID: "network-a",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		mutateFn func(f *v2.FirewallDeployment) *v2.FirewallDeployment
		wantErr  error
	}{
		{
			name: "valid",
			mutateFn: func(f *v2.FirewallDeployment) *v2.FirewallDeployment {
				return f
			},
			wantErr: nil,
		},
		{
			name: "networks are empty",
			mutateFn: func(f *v2.FirewallDeployment) *v2.FirewallDeployment {
				f.Spec.Template.Networks = nil
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.template.networks: Required value: field is required`,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallDeploymentValidator(testr.New(t))

			got := v.ValidateCreate(context.TODO(), tt.mutateFn(valid.DeepCopy()))
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}

func Test_firewallDeploymentValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		newF    *v2.FirewallDeployment
		oldF    *v2.FirewallDeployment
		wantErr error
	}{
		{
			name: "valid",
			newF: &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "firewall",
					Namespace:       "default",
					ResourceVersion: "1",
				},
				Spec: v2.FirewallDeploymentSpec{
					Strategy: v2.StrategyRollingUpdate,
					Template: v2.FirewallSpec{
						Interval:          "10s",
						ControllerURL:     "https://metal-stack.io/controller.img",
						ControllerVersion: "v",
						Image:             "image-a",
						Partition:         "partition-a",
						Project:           "project-a",
						Size:              "size-a",
						Networks:          []string{"internet"},
						EgressRules: []v2.EgressRuleSNAT{
							{
								NetworkID: "network-a",
								IPs:       []string{"1.2.3.4"},
							},
						},
						InternalPrefixes: []string{"1.2.3.0/24"},
						RateLimits: []v2.RateLimit{
							{
								NetworkID: "network-a",
							},
						},
					},
				},
			},
			oldF: &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "firewall",
					Namespace: "default",
				},
				Spec: v2.FirewallDeploymentSpec{
					Strategy: v2.StrategyRollingUpdate,
					Template: v2.FirewallSpec{
						Interval:          "10s",
						ControllerURL:     "https://metal-stack.io/controller.img",
						ControllerVersion: "v",
						Image:             "image-a",
						Partition:         "partition-a",
						Project:           "project-a",
						Size:              "size-a",
						Networks:          []string{"internet"},
						EgressRules: []v2.EgressRuleSNAT{
							{
								NetworkID: "network-a",
								IPs:       []string{"1.2.3.4"},
							},
						},
						InternalPrefixes: []string{"1.2.3.0/24"},
						RateLimits: []v2.RateLimit{
							{
								NetworkID: "network-a",
							},
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallDeploymentValidator(testr.New(t))

			got := v.ValidateUpdate(context.TODO(), tt.oldF, tt.newF)
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}
