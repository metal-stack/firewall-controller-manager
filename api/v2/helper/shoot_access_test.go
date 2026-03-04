package helper

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/metal-lib/pkg/genericcli"
	"github.com/metal-stack/metal-lib/pkg/testcommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
    token: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/token
`),
				},
			}).Build(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewShootAccessHelper(tt.seed, tt.access)
			got, err := h.RESTConfig(context.Background())
			if diff := cmp.Diff(tt.wantErr, err, testcommon.ErrorStringComparer()); diff != "" {
				t.Errorf("error diff (+got -want):\n %s", diff)
			}

			if got != nil {
				assert.Equal(t, "https://shoot-name", got.Host)
			}

			gotRaw, err := h.Raw(context.Background())
			require.NoError(t, err)
			equal, err := genericcli.YamlIsEqual(gotRaw, []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: dGVzdAo=
    server: https://shoot-name
  name: shoot-name
contexts:
- context:
    cluster: shoot-name
    namespace: shoot-namespace
    user: shoot-name
  name: shoot-name
current-context: shoot-name
kind: Config
users:
- name: shoot-name
  user:
    token: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/token
`))
			require.NoError(t, err)
			assert.True(t, equal, string(gotRaw))
		})
	}
}
