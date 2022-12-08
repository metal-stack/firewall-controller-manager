package helper

import (
	"context"
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/tools/clientcmd"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func NewShootConfig(ctx context.Context, seed client.Client, access *v2.ShootAccess) (*rest.Config, error) {
	kubeconfigTemplate := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.GenericKubeconfigSecretName,
			Namespace: access.Namespace,
		},
	}
	err := seed.Get(ctx, client.ObjectKeyFromObject(kubeconfigTemplate), kubeconfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to read generic kubeconfig secret: %w", err)
	}

	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.TokenSecretName,
			Namespace: access.Namespace,
		},
	}
	err = seed.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to read token secret: %w", err)
	}

	kubeconfig := &configv1.Config{}
	err = runtime.DecodeInto(configlatest.Codec, kubeconfigTemplate.Data["kubeconfig"], kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode kubeconfig from generic kubeconfig template: %w", err)
	}

	if len(kubeconfig.AuthInfos) != 1 {
		return nil, fmt.Errorf("parsed generic kubeconfig template does not contain a single user")
	}

	kubeconfig.AuthInfos[0].AuthInfo.TokenFile = ""
	kubeconfig.AuthInfos[0].AuthInfo.Token = string(tokenSecret.Data["token"])

	raw, err := runtime.Encode(configlatest.Codec, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("unable to create rest config from bytes: %w", err)
	}

	return config, nil
}
