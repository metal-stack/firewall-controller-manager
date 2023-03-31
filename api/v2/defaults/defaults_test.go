package defaults

// import (
// 	"context"
// 	"testing"

// 	"github.com/go-logr/logr/testr"
// 	"github.com/google/go-cmp/cmp"
// 	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
// 	"github.com/metal-stack/metal-lib/pkg/testcommon"
// 	"github.com/stretchr/testify/require"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// 	"sigs.k8s.io/controller-runtime/pkg/client/fake"
// )

// func Test_firewallDeploymentDefaulter_Default(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		seed    client.Client
// 		obj     *v2.FirewallDeployment
// 		want    *v2.FirewallDeployment
// 		wantErr error
// 	}{
// 		{
// 			name: "all defaults applied",
// 			seed: fake.NewClientBuilder().WithObjects(&corev1.Secret{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "firewall-controller-seed-access-a",
// 					Namespace: "b",
// 				},
// 				Data: map[string][]byte{
// 					"token":  []byte(`a-token`),
// 					"ca.crt": []byte(`a-ca-crt`),
// 				},
// 			}, &corev1.Secret{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "ssh-secret",
// 					Namespace: "b",
// 				},
// 				Data: map[string][]byte{
// 					"id_rsa.pub": []byte(`ssh-public-key`),
// 				},
// 			}).Build(),
// 			obj: &v2.FirewallDeployment{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "a",
// 					Namespace: "b",
// 				},
// 				Spec: v2.FirewallDeploymentSpec{
// 					Template: v2.FirewallTemplateSpec{
// 						ObjectMeta: metav1.ObjectMeta{
// 							Labels: map[string]string{
// 								"a": "b",
// 							},
// 						},
// 					},
// 				},
// 			},
// 			want: &v2.FirewallDeployment{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      "a",
// 					Namespace: "b",
// 					Annotations: map[string]string{
// 						v2.FirewallUserdataCompatibilityAnnotation: ">=v2.0.0",
// 					},
// 				},
// 				Spec: v2.FirewallDeploymentSpec{
// 					Replicas: 1,
// 					Selector: map[string]string{
// 						"a": "b",
// 					},
// 					Strategy: v2.StrategyRollingUpdate,
// 					Template: v2.FirewallTemplateSpec{
// 						ObjectMeta: metav1.ObjectMeta{
// 							Labels: map[string]string{
// 								"a": "b",
// 							},
// 						},
// 						Spec: v2.FirewallSpec{
// 							Interval:      DefaultFirewallReconcileInterval,
// 							Userdata:      `{"ignition":{"config":{},"security":{"tls":{}},"timeouts":{},"version":"2.3.0"},"networkd":{},"passwd":{},"storage":{"files":[{"filesystem":"root","group":{"id":0},"path":"/etc/firewall-controller/.kubeconfig","user":{"id":0},"contents":{"source":"data:,apiVersion%3A%20v1%0Aclusters%3A%0A-%20cluster%3A%0A%20%20%20%20certificate-authority-data%3A%20YS1jYS1jcnQ%3D%0A%20%20%20%20server%3A%20https%3A%2F%2Fseed-api%0A%20%20name%3A%20b%0Acontexts%3A%0A-%20context%3A%0A%20%20%20%20cluster%3A%20b%0A%20%20%20%20user%3A%20b%0A%20%20name%3A%20b%0Acurrent-context%3A%20b%0Akind%3A%20Config%0Apreferences%3A%20%7B%7D%0Ausers%3A%0A-%20name%3A%20b%0A%20%20user%3A%0A%20%20%20%20token%3A%20a-token%0A","verification":{}},"mode":384}]},"systemd":{"units":[{"enable":true,"enabled":true,"name":"firewall-controller.service"},{"enable":true,"enabled":true,"name":"droptailer.service"}]}}`,
// 							SSHPublicKeys: []string{"ssh-public-key"},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		tt := tt
// 		t.Run(tt.name, func(t *testing.T) {
// 			r, err := NewFirewallDeploymentDefaulter(&DefaulterConfig{
// 				Log:              testr.New(t),
// 				SeedClient:       tt.seed,
// 				Namespace:        tt.obj.Namespace,
// 				SeedAPIServerURL: "https://seed-api",
// 				ShootAccess: &v2.ShootAccess{
// 					GenericKubeconfigSecretName: "generic",
// 					TokenSecretName:             "token",
// 					Namespace:                   "seed-namespace",
// 					APIServerURL:                "https://shot-api",
// 					SSHKeySecretName:            "ssh-secret",
// 				},
// 			})
// 			require.NoError(t, err)

// 			err = r.Default(context.Background(), tt.obj)
// 			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
// 				t.Errorf("error diff (+got -want):\n %s", diff)
// 			}

// 			if diff := cmp.Diff(tt.want, tt.obj); diff != "" {
// 				t.Errorf("diff (+got -want):\n %s", diff)
// 			}
// 		})
// 	}
// }
