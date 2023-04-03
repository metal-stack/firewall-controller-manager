package helper

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang-jwt/jwt/v4"

	"k8s.io/client-go/tools/clientcmd"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

type ShootAccessHelper struct {
	seed      client.Client
	access    *v2.ShootAccess
	tokenPath string
}

func NewShootAccessHelper(seed client.Client, access *v2.ShootAccess) *ShootAccessHelper {
	return &ShootAccessHelper{
		seed:   seed,
		access: access,
	}
}

func (s *ShootAccessHelper) Config(ctx context.Context) (*configv1.Config, error) {
	kubeconfigTemplate := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.access.GenericKubeconfigSecretName,
			Namespace: s.access.Namespace,
		},
	}

	err := s.seed.Get(ctx, client.ObjectKeyFromObject(kubeconfigTemplate), kubeconfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to read generic kubeconfig secret: %w", err)
	}

	kubeconfig := &configv1.Config{}
	err = runtime.DecodeInto(configlatest.Codec, kubeconfigTemplate.Data["kubeconfig"], kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to decode kubeconfig from generic kubeconfig template: %w", err)
	}

	if len(kubeconfig.AuthInfos) != 1 {
		return nil, fmt.Errorf("parsed generic kubeconfig template expects exactly one user")
	}
	if len(kubeconfig.Clusters) != 1 {
		return nil, fmt.Errorf("parsed generic kubeconfig template expects exactly one cluster")
	}
	if len(kubeconfig.Contexts) != 1 {
		return nil, fmt.Errorf("parsed generic kubeconfig template expects exactly one context")
	}

	kubeconfig.Clusters[0].Cluster.Server = s.access.APIServerURL
	kubeconfig.Contexts[0].Context.Namespace = s.access.Namespace
	if s.tokenPath != "" {
		kubeconfig.AuthInfos[0].AuthInfo.TokenFile = s.tokenPath
	}

	return kubeconfig, nil
}

func (s *ShootAccessHelper) Raw(ctx context.Context) ([]byte, error) {
	config, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := runtime.Encode(configlatest.Codec, config)
	if err != nil {
		return nil, fmt.Errorf("unable to encode kubeconfig: %w", err)
	}

	return raw, nil
}

func (s *ShootAccessHelper) RESTConfig(ctx context.Context) (*rest.Config, error) {
	raw, err := s.Raw(ctx)
	if err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("unable to create rest config from bytes: %w", err)
	}

	return restConfig, nil
}

func (s *ShootAccessHelper) Client(ctx context.Context) (client.Client, error) {
	config, err := s.RESTConfig(ctx)
	if err != nil {
		return nil, err
	}

	client, err := controllerclient.New(config, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create shoot client: %w", err)
	}

	return client, err
}

func (s *ShootAccessHelper) K8sVersion(ctx context.Context) (*semver.Version, error) {
	config, err := s.RESTConfig(ctx)
	if err != nil {
		return nil, err
	}

	v, err := determineK8sVersion(config)
	if err != nil {
		return nil, err
	}

	return v, err
}

func (s *ShootAccessHelper) ReadTokenSecret(ctx context.Context) (*time.Time, string, error) {
	tokenSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.access.TokenSecretName,
			Namespace: s.access.Namespace,
		},
	}

	err := s.seed.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret)
	if err != nil {
		return nil, "", fmt.Errorf("unable to read token secret: %w", err)
	}

	token := string(tokenSecret.Data["token"])

	claims := &jwt.RegisteredClaims{}
	_, _, err = new(jwt.Parser).ParseUnverified(token, claims)
	if err != nil {
		return nil, "", fmt.Errorf("shoot access token is not parsable: %w", err)
	}

	if claims.ExpiresAt != nil {
		return &claims.ExpiresAt.Time, token, nil
	}

	return nil, token, nil
}

type ShootAccessTokenUpdater struct {
	s *ShootAccessHelper
}

func NewShootAccessTokenUpdater(s *ShootAccessHelper, tokenDir string) (*ShootAccessTokenUpdater, error) {
	file, err := os.Create(path.Join(tokenDir, fmt.Sprintf("%s-token", v2.FirewallControllerManager)))
	if err != nil {
		return nil, fmt.Errorf("unable to file for shoot token: %w", err)
	}

	s.tokenPath = file.Name()

	err = file.Close()
	if err != nil {
		return nil, fmt.Errorf("unable to close file for shoot token: %w", err)
	}

	return &ShootAccessTokenUpdater{
		s: s,
	}, nil
}

func (s *ShootAccessTokenUpdater) UpdateContinuously(log logr.Logger, stop context.Context) error {
	log.Info("updating token file", "path", s.s.tokenPath)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, token, err := s.s.ReadTokenSecret(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("unable to read token secret: %w", err)
	}

	err = os.WriteFile(s.s.tokenPath, []byte(token), 0600)
	if err != nil {
		return fmt.Errorf("unable to write token file: %w", err)
	}

	log.Info("updated token file successfully, next update in 5 minutes")

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Info("updating token file", "path", s.s.tokenPath)

				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				_, token, err := s.s.ReadTokenSecret(ctx)
				cancel()
				if err != nil {
					log.Error(err, "unable to read token secret")
					continue
				}

				err = os.WriteFile(s.s.tokenPath, []byte(token), 0600)
				if err != nil {
					log.Error(err, "unable to update token file")
					continue
				}

				log.Info("updated token file successfully, next update in 5 minutes")
			case <-stop.Done():
				return
			}
		}
	}()

	return nil
}
