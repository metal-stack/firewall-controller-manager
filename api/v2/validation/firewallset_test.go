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

func Test_firewalSetValidator_ValidateCreate(t *testing.T) {
	valid := &v2.FirewallSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall",
			Namespace: "default",
		},
		Spec: v2.FirewallSetSpec{
			Selector: map[string]string{
				"purpose": "shoot-firewall",
			},
			Template: v2.FirewallTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"purpose": "shoot-firewall",
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
		},
	}

	tests := []struct {
		name     string
		mutateFn func(f *v2.FirewallSet) *v2.FirewallSet
		wantErr  error
	}{
		{
			name: "valid",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				return f
			},
			wantErr: nil,
		},
		{
			name: "networks are empty",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.Spec.Template.Spec.Networks = nil
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.template.spec.networks: Required value: field is required`,
				},
			},
		},
		{
			name: "labels must match",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.Spec.Selector = map[string]string{
					"a": "b",
				}
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.template.metadata.labels: Invalid value: map[string]string{"purpose":"shoot-firewall"}: ` + "`selector` does not match template `labels`",
				},
			},
		},
		{
			name: "labels must match",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.Spec.Selector = map[string]string{
					"does": "not-match",
				}
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.template.metadata.labels: Invalid value: map[string]string{"purpose":"shoot-firewall"}: ` + "`selector` does not match template `labels`",
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallSetValidator(testr.New(t))

			got := v.ValidateCreate(context.Background(), tt.mutateFn(valid.DeepCopy()))
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}

func Test_firewallSetValidator_ValidateUpdate(t *testing.T) {
	valid := &v2.FirewallSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall",
			Namespace: "default",
		},
		Spec: v2.FirewallSetSpec{
			Selector: map[string]string{
				"purpose": "shoot-firewall",
				"a":       "b",
			},
			Template: v2.FirewallTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"purpose": "shoot-firewall",
						"a":       "b",
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
		},
	}

	tests := []struct {
		name     string
		mutateFn func(f *v2.FirewallSet) *v2.FirewallSet
		wantErr  error
	}{
		{
			name: "valid",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.ResourceVersion = "1"
				return f
			},
			wantErr: nil,
		},
		{
			name: "prevent selector update",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.ResourceVersion = "1"
				f.Spec.Selector = map[string]string{
					"purpose": "shoot-firewall",
				}
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.selector: Invalid value: map[string]string{"purpose":"shoot-firewall"}: field is immutable`,
				},
			},
		},
		{
			name: "prevent updating project in template spec",
			mutateFn: func(f *v2.FirewallSet) *v2.FirewallSet {
				f.ResourceVersion = "1"
				f.Spec.Template.Spec.Project = "new"
				return f
			},
			wantErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Message: ` "firewall" is invalid: spec.template.spec.project: Invalid value: "new": field is immutable`,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			v := NewFirewallSetValidator(testr.New(t))

			got := v.ValidateUpdate(context.Background(), valid.DeepCopy(), tt.mutateFn(valid.DeepCopy()))
			if diff := cmp.Diff(tt.wantErr, got, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}
		})
	}
}
