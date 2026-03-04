package deployment

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_controller_updateInfrastructureStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	ctx := context.Background()
	log := testr.New(t)

	testNamespace := "shoot--abcdef--mycluster1"

	tests := []struct {
		name           string
		objs           []client.Object
		ownedFirewalls []*v2.Firewall
		want           []client.Object
		wantErr        error
	}{
		{
			name:    "no infrastructure present",
			objs:    nil,
			wantErr: nil,
		},
		{
			name: "infrastructure is present, egress cidrs were not yet set (acl extension gets annotated)",
			objs: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase": "foo",
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Extension",
						"metadata": map[string]any{
							"name":            "acl",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
					},
				},
			},
			ownedFirewalls: []*v2.Firewall{
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.1"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.4"},
							},
						},
					},
				},
			},
			want: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "1000",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32"},
						},
					},
				},
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Extension",
						"metadata": map[string]any{
							"name":            "acl",
							"namespace":       testNamespace,
							"resourceVersion": "1000",
							"annotations": map[string]any{
								"gardener.cloud/operation": "reconcile",
							},
						},
						"status": nil,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "infrastructure is present, egress cidrs have already been set but not up-to-date",
			objs: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.2.3.4/32", "2.3.4.5/32"},
						},
					},
				},
			},
			ownedFirewalls: []*v2.Firewall{
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.1"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.4"},
							},
						},
					},
				},
			},
			want: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "1000",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32"},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "infrastructure is present, egress cidrs have already been set and are up-to-date",
			objs: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32"},
						},
					},
				},
			},
			ownedFirewalls: []*v2.Firewall{
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.1"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.4"},
							},
						},
					},
				},
			},
			want: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32"},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "infrastructure has egress rules",
			objs: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
									"egressRules": []any{
										map[string]any{
											"ips": []any{"3.4.5.6"},
										},
									},
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32"},
						},
					},
				},
			},
			ownedFirewalls: []*v2.Firewall{
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.1"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.4"},
							},
						},
					},
				},
			},
			want: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "1000",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
									"egressRules": []any{
										map[string]any{
											"ips": []any{"3.4.5.6"},
										},
									},
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.1/32", "3.4.5.6/32"},
						},
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "skip update on different order of ip elements in slice",
			objs: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.2/32", "1.1.1.1/32"},
						},
					},
				},
			},
			ownedFirewalls: []*v2.Firewall{
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.1"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.4"},
							},
						},
					},
				},
				{
					Status: v2.FirewallStatus{
						FirewallNetworks: []v2.FirewallNetwork{
							{
								NetworkType: new("external"),
								IPs:         []string{"1.1.1.2"},
							},
							{
								NetworkType: new("underlay"),
								IPs:         []string{"10.8.0.5"},
							},
						},
					},
				},
			},
			want: []client.Object{
				&unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "extensions.gardener.cloud/v1alpha1",
						"kind":       "Infrastructure",
						"metadata": map[string]any{
							"name":            "mycluster1",
							"namespace":       testNamespace,
							"resourceVersion": "999",
						},
						"spec": map[string]any{
							"providerConfig": map[string]any{
								"apiVersion": "metal.provider.extensions.gardener.cloud/v1alpha1",
								"firewall": map[string]any{
									"controllerVersion": "auto",
								},
							},
						},
						"status": map[string]any{
							"phase":       "foo",
							"egressCIDRs": []any{"1.1.1.2/32", "1.1.1.1/32"},
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objs...).WithStatusSubresource(tt.objs...).Build()

			cc, err := config.New(&config.NewControllerConfig{
				SeedClient:     c,
				SeedNamespace:  testNamespace,
				SkipValidation: true,
			})
			require.NoError(t, err)

			ctrl := &controller{
				log: log,
				c:   cc,
			}

			err = ctrl.updateInfrastructureStatus(&controllers.Ctx[*v2.FirewallDeployment]{
				Ctx: ctx,
				Log: log,
			}, "mycluster1", tt.ownedFirewalls)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			for _, want := range tt.want {
				u := unstructured.Unstructured{}
				u.SetGroupVersionKind(want.GetObjectKind().GroupVersionKind())

				err = c.Get(ctx, client.ObjectKeyFromObject(want), &u)
				require.NoError(t, err)

				if diff := cmp.Diff(want, &u); diff != "" {
					t.Errorf("diff (+got -want):\n %s", diff)
				}
			}
		})
	}
}

func Test_extractInfrastructureNameFromSeedNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
		wantBool  bool
	}{
		{
			name:      "default namespace not considered",
			namespace: "default",
			want:      "",
			wantBool:  false,
		},
		{
			name:      "correctly extract from gardener namespace scheme",
			namespace: "shoot--abcdef--mycluster1",
			want:      "mycluster1",
			wantBool:  true,
		},
		{
			name:      "incorrect namespace scheme #1",
			namespace: "shoot--abcdef",
			want:      "",
			wantBool:  false,
		},
		{
			name:      "another double-hyphen in cluster name",
			namespace: "shoot--abcdef--myclust--er1",
			want:      "myclust--er1",
			wantBool:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotBool := extractInfrastructureNameFromSeedNamespace(tt.namespace)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
			if diff := cmp.Diff(gotBool, tt.wantBool); diff != "" {
				t.Errorf("diff (+got -want):\n %s", diff)
			}
		})
	}
}
