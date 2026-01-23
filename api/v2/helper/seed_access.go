package helper

import (
	"context"
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	configlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	configv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func EnsureFirewallControllerRBAC(ctx context.Context, seedConfig, shootConfig *rest.Config, deploy *v2.FirewallDeployment, shootNamespace string, shootAccess *v2.ShootAccess) error {
	err := ensureSeedRBAC(ctx, seedConfig, deploy, shootAccess)
	if err != nil {
		return fmt.Errorf("unable to ensure seed rbac: %w", err)
	}

	err = ensureShootRBAC(ctx, shootConfig, shootNamespace, deploy)
	if err != nil {
		return fmt.Errorf("unable to ensure shoot rbac: %w", err)
	}

	return nil
}

func ensureSeedRBAC(ctx context.Context, seedConfig *rest.Config, deploy *v2.FirewallDeployment, shootAccess *v2.ShootAccess) error {
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

	seed, err := controllerclient.New(seedConfig, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return fmt.Errorf("unable to create seed client: %w", err)
	}

	_, err = controllerutil.CreateOrUpdate(ctx, seed, serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			"token-invalidator.resources.gardener.cloud/skip": "true",
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account: %w", err)
	}

	serviceAccountSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deploy.Namespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, seed, serviceAccountSecret, func() error {
		serviceAccountSecret.Annotations = map[string]string{
			"kubernetes.io/service-account.name": serviceAccount.Name,
		}
		serviceAccountSecret.Type = corev1.SecretTypeServiceAccountToken
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account token secret: %w", err)
	}

	var shootAccessSecretNames []string
	if shootAccess.GenericKubeconfigSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, shootAccess.GenericKubeconfigSecretName)
	}
	if shootAccess.TokenSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, shootAccess.TokenSecretName)
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

	serviceAccountSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: shootNamespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, shoot, serviceAccountSecret, func() error {
		serviceAccountSecret.Annotations = map[string]string{
			"kubernetes.io/service-account.name": serviceAccount.Name,
		}
		serviceAccountSecret.Type = corev1.SecretTypeServiceAccountToken
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account token secret: %w", err)
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
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
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

	cl, err := controllerclient.New(c.Config, controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %w", err)
	}

	saSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: c.Namespace,
		},
	}
	err = cl.Get(c.Ctx, controllerclient.ObjectKeyFromObject(saSecret), saSecret, &controllerclient.GetOptions{})
	if err != nil {
		return nil, err
	}

	token = string(saSecret.Data["token"])
	ca = saSecret.Data["ca.crt"]

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
