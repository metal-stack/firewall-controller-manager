package helper

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewShootClient(t *testing.T) {
	tests := []struct {
		name    string
		seed    client.Client
		access  *v2.ShootAccess
		wantErr error
	}{
		{
			name: "secrets from gardener",
			access: &v2.ShootAccess{
				GenericKubeconfigSecretName: "generic-token-kubeconfig",
				TokenSecretName:             "shoot-access-token",
				Namespace:                   "shoot-namespace",
				APIServerURL:                "https://shoot-name",
			},
			seed: fake.NewClientBuilder().WithObjects(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "generic-token-kubeconfig",
					Namespace: "shoot-namespace",
				},
				Data: map[string][]byte{
					"kubeconfig": []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: dGVzdAo=
    server: https://kube-apiserver
  name: shoot-name
contexts:
- context:
    cluster: shoot-name
    user: shoot-name
  name: shoot-name
current-context: shoot-name
kind: Config
preferences: {}
users:
- name: shoot-name
  user:
    tokenFile: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/token
`)},
			}, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot-access-token",
					Namespace: "shoot-namespace",
				},
				StringData: map[string]string{
					"token": "a-token",
				},
			}).Build(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewShootClient(context.Background(), tt.seed, tt.access)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			assert.Equal(t, got.Host, "https://kube-apiserver")
		})
	}
}
