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
		name           = "firewall-controller-seed-access"
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
