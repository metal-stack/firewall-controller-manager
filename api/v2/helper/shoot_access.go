package helper

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang-jwt/jwt/v4"

	"k8s.io/client-go/tools/clientcmd"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func NewShootConfig(ctx context.Context, seed client.Client, access *v2.ShootAccess) (*time.Time, *rest.Config, error) {
	kubeconfigTemplate := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.GenericKubeconfigSecretName,
			Namespace: access.Namespace,
		},
	}
	err := seed.Get(ctx, client.ObjectKeyFromObject(kubeconfigTemplate), kubeconfigTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read generic kubeconfig secret: %w", err)
	}

	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      access.TokenSecretName,
			Namespace: access.Namespace,
		},
	}
	err = seed.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read token secret: %w", err)
	}

	kubeconfig := &configv1.Config{}
	err = runtime.DecodeInto(configlatest.Codec, kubeconfigTemplate.Data["kubeconfig"], kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode kubeconfig from generic kubeconfig template: %w", err)
	}

	if len(kubeconfig.AuthInfos) != 1 {
		return nil, nil, fmt.Errorf("parsed generic kubeconfig template does not contain a single user")
	}

	token := string(tokenSecret.Data["token"])

	kubeconfig.AuthInfos[0].AuthInfo.TokenFile = ""
	kubeconfig.AuthInfos[0].AuthInfo.Token = token

	claims := &jwt.RegisteredClaims{}
	_, _, err = new(jwt.Parser).ParseUnverified(token, claims)
	if err != nil {
		return nil, nil, fmt.Errorf("shoot access token is not parsable: %w", err)
	}

	raw, err := runtime.Encode(configlatest.Codec, kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create rest config from bytes: %w", err)
	}

	if claims.ExpiresAt != nil {
		return &claims.ExpiresAt.Time, config, nil
	}

	return nil, config, nil
}

func ShutdownOnTokenExpiration(log logr.Logger, expiresAt *time.Time, stop context.Context) {
	if expiresAt == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if time.Now().Add(10 * time.Minute).Before(*expiresAt) {
					log.Info("token is not yet expiring, continue to run until around 10 minutes before expiration", "expiring-at", expiresAt.String())
					continue
				}

				log.Info("token is expiring, shutting down and restart to renew clients")

				ctx, cancel := context.WithTimeout(stop, 10*time.Second)
				defer cancel()
				ctx.Done()

				return
			}
		}
	}()

	return
}
