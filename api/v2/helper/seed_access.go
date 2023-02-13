package helper

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func EnsureFirewallControllerRBAC(ctx context.Context, k8sVersion *semver.Version, seed client.Client, deploy *v2.FirewallDeployment, shootAccess *v2.ShootAccess) error {
	var (
		name           = seedAccessResourceName(deploy)
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}
	)

	_, err := controllerutil.CreateOrUpdate(ctx, seed, serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			"token-invalidator.resources.gardener.cloud/skip": "true",
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account: %w", err)
	}

	if VersionGreaterOrEqual125(k8sVersion) {
		serviceAccountSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, seed, serviceAccountSecret, func() error {
			serviceAccountSecret.Annotations = map[string]string{
				"kubernetes.io/service-account.name": serviceAccount.Name,
			}
			serviceAccountSecret.Type = corev1.SecretTypeServiceAccountToken
			return nil
		})
		if err != nil {
			return fmt.Errorf("error ensuring service account token secret: %w", err)
		}
	}

	var shootAccessSecretNames []string
	if shootAccess.GenericKubeconfigSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, shootAccess.GenericKubeconfigSecretName)
	}
	if shootAccess.TokenSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, shootAccess.TokenSecretName)
	}
	if shootAccess.SSHKeySecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, shootAccess.SSHKeySecretName)
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deploy.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, seed, role, func() error {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{v2.GroupVersion.Group},
				Resources: []string{"firewalls"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{v2.GroupVersion.Group},
				Resources: []string{"firewalls/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				Verbs:         []string{"get", "list", "watch"},
				ResourceNames: shootAccessSecretNames,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role: %w", err)
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deploy.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, seed, roleBinding, func() error {
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     name,
		}
		roleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role binding: %w", err)
	}

	return nil
}

type SeedAccessConfig struct {
	Ctx          context.Context
	Client       client.Client
	K8sVersion   *semver.Version
	Namespace    string
	ApiServerURL string
	Deployment   *v2.FirewallDeployment
}

func (s *SeedAccessConfig) validate() error {
	if s.Ctx == nil {
		return fmt.Errorf("context must be specified")
	}
	if s.Client == nil {
		return fmt.Errorf("client must be specified")
	}
	if s.K8sVersion == nil {
		return fmt.Errorf("k8s version must be specified")
	}
	if s.Namespace == "" {
		return fmt.Errorf("namespace must be specified")
	}
	if s.ApiServerURL == "" {
		return fmt.Errorf("api server url must be specified")
	}
	if s.Deployment == nil {
		return fmt.Errorf("deployment must be specified")
	}

	return nil
}

func SeedAccessKubeconfig(c *SeedAccessConfig) ([]byte, error) {
	var (
		name  = seedAccessResourceName(c.Deployment)
		ca    []byte
		token string
	)

	err := c.validate()
	if err != nil {
		return nil, err
	}

	if VersionGreaterOrEqual125(c.K8sVersion) {
		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: c.Namespace,
			},
		}
		err := c.Client.Get(c.Ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
		if err != nil {
			return nil, err
		}

		token = string(saSecret.Data["token"])
		ca = saSecret.Data["ca.crt"]
	} else {
		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: c.Namespace,
			},
		}
		err := c.Client.Get(c.Ctx, client.ObjectKeyFromObject(sa), sa, &client.GetOptions{})
		if err != nil {
			return nil, err
		}

		if len(sa.Secrets) == 0 {
			return nil, fmt.Errorf("service account %q contains no valid token secret", sa.Name)
		}

		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sa.Secrets[0].Name,
				Namespace: c.Namespace,
			},
		}
		err = c.Client.Get(c.Ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
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
		CurrentContext: c.Namespace,
		Clusters: []configv1.NamedCluster{
			{
				Name: c.Namespace,
				Cluster: configv1.Cluster{
					CertificateAuthorityData: ca,
					Server:                   c.ApiServerURL,
				},
			},
		},
		Contexts: []configv1.NamedContext{
			{
				Name: c.Namespace,
				Context: configv1.Context{
					Cluster:  c.Namespace,
					AuthInfo: c.Namespace,
				},
			},
		},
		AuthInfos: []configv1.NamedAuthInfo{
			{
				Name: c.Namespace,
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

func seedAccessResourceName(deploy *v2.FirewallDeployment) string {
	return "firewall-controller-seed-access-" + deploy.Name
}
