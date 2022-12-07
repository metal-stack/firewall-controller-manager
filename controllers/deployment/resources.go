package deployment

import (
	"fmt"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *controller) ensureFirewallControllerRBAC(r *controllers.Ctx[*v2.FirewallDeployment]) error {
	r.Log.Info("ensuring firewall controller rbac")

	var err error
	defer func() {
		if err != nil {
			r.Log.Error(err, "unable to ensure firewall controller rbac")

			cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionFalse, "Error", fmt.Sprintf("RBAC resources could not be provisioned %s", err))
			r.Target.Status.Conditions.Set(cond)

			return
		}

		cond := v2.NewCondition(v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, "Provisioned", "RBAC provisioned successfully.")
		r.Target.Status.Conditions.Set(cond)
	}()

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Seed, serviceAccount, func() error {
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

		_, err := controllerutil.CreateOrUpdate(r.Ctx, c.Seed, serviceAccountSecret, func() error {
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
	if c.ShootKubeconfigSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, c.ShootKubeconfigSecretName)
	}
	if c.ShootTokenSecretName != "" {
		shootAccessSecretNames = append(shootAccessSecretNames, c.ShootTokenSecretName)
	}

	role := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Seed, role, func() error {
		role.Rules = []rbac.PolicyRule{
			{
				APIGroups: []string{v2.GroupVersion.String()},
				Resources: []string{"firewall"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{v2.GroupVersion.String()},
				Resources: []string{"firewall/status"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups:     []string{"core/v1"},
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

	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access",
			Namespace: c.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(r.Ctx, c.Seed, roleBinding, func() error {
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
