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

func TestNewShootConfig(t *testing.T) {
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
				Data: map[string][]byte{
					"token": []byte(`eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.NHVaYe26MbtOYhSKkoKYdFVomg4i8ZJd8_-RU8VNbftc4TSMb4bXP3l3YlNWACwyXPGffz5aXHc6lty1Y2t4SWRqGteragsVdZufDn5BlnJl9pdR_kdVFUsra2rWKEofkZeIC4yWytE58sMIihvo9H1ScmmVwBcQP6XETqYd0aSHp1gOa9RdUPDvoXQ5oqygTqVtxaDr6wUFKrKItgBMzWIdNZ6y7O9E0DhEPTbE9rfBo6KTFsHAZnMg4k68CDp2woYIaXbmYTWcvbzIuHO7_37GT79XdIwkm95QJ7hYC9RiwrV7mesbY4PAahERJawntho0my942XheVLmGwLMBkQ`),
				},
			}).Build(),
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			_, _, got, err := NewShootConfig(context.Background(), tt.seed, tt.access)
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			if got != nil {
				assert.Equal(t, got.Host, "https://shoot-name")
			}
		})
	}
}
