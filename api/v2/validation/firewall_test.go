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

func Test_firewallValidator_ValidateCreate(t *testing.T) {
	valid := &v2.Firewall{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-123",
			Namespace: "default",
			Annotations: map[string]string{
				v2.FirewallWeightAnnotation:                 "100",
				v2.FirewallNoControllerConnectionAnnotation: "true",
			},
		},
		Spec: v2.FirewallSpec{
			Interval:                "10s",
			ControllerURL:           "https://metal-stack.io/controller.img",
			ControllerVersion:       "v",
			NftablesExporterURL:     "http://exporter.tar.gz",
			NftablesExporterVersion: "v1.0.0",
			Image:                   "image-a",
			Partition:               "partition-a",
			Project:                 "project-a",
			Size:                    "size-a",
			Networks:                []string{"internet"},
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
	}

	tests := []struct {
		name     string
		mutateFn func(f *v2.Firewall) *v2.Firewall
		wantErr  error
	}{
		{
			name: "valid",
			mutateFn: func(f *v2.Firewall) *v2.Firewall {
				return f
			},
			wantErr: nil,
		},
		{
			name: "networks are empty",
			mutateFn: func(f *v2.Firewall) *v2.Firewall {
				f.Spec.Networks = nil
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall-123" is invalid: spec.networks: Required value: field is required`,
				},
			},
		},
		{
			name: "bad weight annotation",
			mutateFn: func(f *v2.Firewall) *v2.Firewall {
				f.Annotations[v2.FirewallWeightAnnotation] = "foo"
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall-123" is invalid: metadata.annotations: Invalid value: "foo": value of "firewall.metal-stack.io/weight" must be parsable as int`,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallValidator(testr.New(t))

			got := v.ValidateCreate(context.Background(), tt.mutateFn(valid.DeepCopy()))
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}

func Test_firewallValidator_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name    string
		newF    *v2.Firewall
		oldF    *v2.Firewall
		wantErr error
	}{
		{
			name: "valid",
			newF: &v2.Firewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "firewall-123",
					Namespace:       "default",
					ResourceVersion: "1",
					Annotations: map[string]string{
						v2.FirewallNoControllerConnectionAnnotation: "true",
					},
				},
				Spec: v2.FirewallSpec{
					Interval:                "10s",
					ControllerURL:           "https://metal-stack.io/controller.img",
					ControllerVersion:       "v",
					NftablesExporterURL:     "http://exporter.tar.gz",
					NftablesExporterVersion: "v1.0.0",
					Image:                   "image-a",
					Partition:               "partition-a",
					Project:                 "project-a",
					Size:                    "size-a",
					Networks:                []string{"internet"},
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
			oldF: &v2.Firewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "firewall-123",
					Namespace: "default",
				},
				Spec: v2.FirewallSpec{
					Interval:                "10s",
					ControllerURL:           "https://metal-stack.io/controller.img",
					ControllerVersion:       "v",
					NftablesExporterURL:     "http://exporter.tar.gz",
					NftablesExporterVersion: "v1.0.0",
					Image:                   "image-a",
					Partition:               "partition-a",
					Project:                 "project-a",
					Size:                    "size-a",
					Networks:                []string{"internet"},
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
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallValidator(testr.New(t))

			got := v.ValidateUpdate(context.Background(), tt.oldF, tt.newF)
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}
