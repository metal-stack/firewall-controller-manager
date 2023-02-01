package helper

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SeedAccessKubeconfig(ctx context.Context, c client.Client, k8sVersion *semver.Version, namespace, apiServerURL string) ([]byte, error) {
	var (
		ca    []byte
		token string
	)

	if VersionGreaterOrEqual125(k8sVersion) {
		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: namespace,
			},
		}
		err := c.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return nil, err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	} else {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: namespace,
			},
		}
		err := c.Get(ctx, client.ObjectKeyFromObject(sa), sa, &client.GetOptions{})
		if err != nil {
			return nil, err
		}

		if len(sa.Secrets) == 0 {
			return nil, fmt.Errorf("service account %q contains no valid token secret", sa.Name)
		}

		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sa.Secrets[0].Name,
				Namespace: namespace,
			},
		}
		err = c.Get(ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return nil, err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	}

	if token == "" {
		return nil, fmt.Errorf("no token was created")
	}

	config := &configv1.Config{
		CurrentContext: namespace,
		Clusters: []configv1.NamedCluster{
			{
				Name: namespace,
				Cluster: configv1.Cluster{
					CertificateAuthorityData: ca,
					Server:                   apiServerURL,
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: namespace,
				Context: configv1.Context{
					Cluster:  namespace,
					AuthInfo: namespace,
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name: namespace,
				AuthInfo: configv1.AuthInfo{
					Token: token,
				},
			},
		},
	}

	kubeconfig, err := runtime.Encode(configlatest.Codec, config)
	if err != nil {
		return nil, fmt.Errorf("unable to encode kubeconfig for firewall-controller seed access: %w", err)
	}

	return kubeconfig, nil
}
