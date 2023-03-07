package helper

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func EnsureFirewallControllerRBAC(ctx context.Context, seedConfig *rest.Config, deploy *v2.FirewallDeployment, shootNamespace string, shootAccess *v2.ShootAccess) error {
	seed, err := ensureSeedRBAC(ctx, seedConfig, deploy, shootAccess)
	if err != nil {
		return fmt.Errorf("unable to ensure seed rbac: %w", err)
	}

	_, _, shootConfig, err := NewShootConfig(ctx, seed, shootAccess)
	if err != nil {
		return fmt.Errorf("unable to create shoot client: %w", err)
	}

	err = ensureShootRBAC(ctx, shootConfig, shootNamespace, deploy)
	if err != nil {
		return fmt.Errorf("unable to ensure shoot rbac: %w", err)
	}

	return nil
}

func ensureSeedRBAC(ctx context.Context, seedConfig *rest.Config, deploy *v2.FirewallDeployment, shootAccess *v2.ShootAccess) (client.Client, error) {
	var (
		name           = seedAccessResourceName(deploy)
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: deploy.Namespace,
			},
		}
	)

	k8sVersion, err := determineK8sVersion(seedConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to determine seed k8s version: %w", err)
	}

	seed, err := controllerclient.New(seedConfig, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create seed client: %w", err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, seed, serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			"token-invalidator.resources.gardener.cloud/skip": "true",
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error ensuring service account: %w", err)
	}

	if versionGreaterOrEqual125(k8sVersion) {
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
			return nil, fmt.Errorf("error ensuring service account token secret: %w", err)
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
		return nil, fmt.Errorf("error ensuring role: %w", err)
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
		return nil, fmt.Errorf("error ensuring role binding: %w", err)
	}

	return seed, nil
}

func ensureShootRBAC(ctx context.Context, shootConfig *rest.Config, shootNamespace string, deploy *v2.FirewallDeployment) error {
	var (
		name           = shootAccessResourceName(deploy)
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: shootNamespace,
			},
		}
		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}
	)

	k8sVersion, err := determineK8sVersion(shootConfig)
	if err != nil {
		return fmt.Errorf("unable to determine shoot k8s version: %w", err)
	}

	shoot, err := controllerclient.New(shootConfig, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("unable to create shoot client: %w", err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, shoot, serviceAccount, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account: %w", err)
	}

	if versionGreaterOrEqual125(k8sVersion) {
		serviceAccountSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: shootNamespace,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, shoot, serviceAccount, func() error {
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

	_, err = controllerutil.CreateOrUpdate(ctx, shoot, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "secrets", "services"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io", ""},
				Resources: []string{"customresourcedefinitions", "services", "endpoints"},
				Verbs:     []string{"get", "create", "update", "list", "watch"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"networkpolicies"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"metal-stack.io"},
				Resources: []string{"firewalls", "firewalls/status", "clusterwidenetworkpolicies", "clusterwidenetworkpolicies/status"},
				Verbs:     []string{"list", "get", "update", "patch", "create", "delete", "watch"},
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring cluster role: %w", err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, shoot, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     name,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      name,
				Namespace: shootNamespace,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring cluster role binding: %w", err)
	}

	return nil
}

func determineK8sVersion(config *rest.Config) (*semver.Version, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("unable to create discovery client: %w", err)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to discover server version: %w", err)
	}

	k8sVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to parse kubernetes version version: %w", err)
	}

	return k8sVersion, nil
}

func versionGreaterOrEqual125(v *semver.Version) bool {
	constraint, err := semver.NewConstraint(">=v1.25.0")
	if err != nil {
		return false
	}

	return constraint.Check(v)
}

type AccessConfig struct {
	Ctx          context.Context
	Config       *rest.Config
	Namespace    string
	ApiServerURL string
	Deployment   *v2.FirewallDeployment
	ForShoot     bool
}

func (s *AccessConfig) validate() error {
	if s.Ctx == nil {
		return fmt.Errorf("context must be specified")
	}
	if s.Config == nil {
		return fmt.Errorf("client config must be specified")
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

func GetAccessKubeconfig(c *AccessConfig) ([]byte, error) {
	var (
		name  = seedAccessResourceName(c.Deployment)
		ca    []byte
		token string
	)

	if c.ForShoot {
		name = shootAccessResourceName(c.Deployment)
	}

	err := c.validate()
	if err != nil {
		return nil, err
	}

	k8sVersion, err := determineK8sVersion(c.Config)
	if err != nil {
		return nil, fmt.Errorf("unable to determine k8s version: %w", err)
	}

	cl, err := controllerclient.New(c.Config, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	if versionGreaterOrEqual125(k8sVersion) {
		saSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: c.Namespace,
			},
		}
		err := cl.Get(c.Ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
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
		err := cl.Get(c.Ctx, client.ObjectKeyFromObject(sa), sa, &client.GetOptions{})
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
		err = cl.Get(c.Ctx, client.ObjectKeyFromObject(saSecret), saSecret, &client.GetOptions{})
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

func shootAccessResourceName(deploy *v2.FirewallDeployment) string {
	return "firewall-controller-shoot-access-" + deploy.Name
}
