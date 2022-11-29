package deployment

import (
	"context"
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) ensureFirewallControllerRBAC(ctx context.Context) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, c.Seed, serviceAccount, func() error {
		serviceAccount.Labels = map[string]string{
			"token-invalidator.resources.gardener.cloud/skip": "true",
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring service account: %w", err)
	}

	if controllers.VersionGreaterOrEqual125(c.K8sVersion) {
		serviceAccountSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "firewall-controller-seed-access",
				Namespace: c.Namespace,
			},
		}

		_, err := controllerutil.CreateOrUpdate(ctx, c.Seed, serviceAccountSecret, func() error {
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

	role := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.Seed, role, func() error {
		role.Rules = []rbac.PolicyRule{
			{
				APIGroups: []string{v2.GroupVersion.String()},
				Resources: []string{"firewall"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{v2.GroupVersion.String()},
				Resources: []string{"firewall/status"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role: %w", err)
	}

	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, c.Seed, roleBinding, func() error {
		roleBinding.RoleRef = rbac.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "Role",
			Name:     "firewall-controller-seed-access",
		}
		roleBinding.Subjects = []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "firewall-controller-seed-access",
				Namespace: c.Namespace,
			},
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error ensuring role binding: %w", err)
	}

	return nil
}
