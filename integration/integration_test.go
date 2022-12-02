package controllers_test

import (
	"context"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metalfirewall "github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
	metalclient "github.com/metal-stack/metal-go/test/client"
)

var _ = Describe("firewall deployment controller", Ordered, func() {
	Context("the good case", Ordered, func() {
		var (
			ctx      = context.Background()
			timeout  = 5 * time.Second
			interval = 250 * time.Millisecond

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: namespaceName,
				},
			}

			deployment = &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: namespaceName,
				},
				Spec: v2.FirewallDeploymentSpec{
					Template: v2.FirewallSpec{
						Size:              "n1-medium-x86",
						Project:           "project-a",
						Partition:         "partition-a",
						Image:             "firewall-ubuntu-2.0",
						Networks:          []string{"internet"},
						ControllerURL:     "http://controller.tar.gz",
						ControllerVersion: "v0.0.1",
					},
				},
			}

			fakeTokenSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "firewall-controller-seed-access",
					Namespace: namespaceName,
					Annotations: map[string]string{
						"kubernetes.io/service-account.name": "firewall-controller-seed-access",
					},
				},
				StringData: map[string]string{
					"token":  "a-token",
					"ca.crt": "ca-crt",
				},
				Type: corev1.SecretTypeServiceAccountToken,
			}
		)

		_, metalClient = metalclient.NewMetalMockClient(&metalclient.MetalMockFns{
			Firewall: func(m *mock.Mock) {
				m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{}}, nil)
				m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil)
			},
			Network: func(m *mock.Mock) {
				m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil)
			},
		})

		BeforeAll(func() {
			By("creating a firewall deployment")

			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
			// we need to fake the secret as there is no kube-controller-manager in the
			// envtest setup which can issue a long-lived token for the secret
			Expect(k8sClient.Create(ctx, fakeTokenSecret)).To(Succeed())
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
		})

		Context("everything comes up healthy", func() {
			It("should default the update strategy to rolling update, so the mutating webhook is working", func() {
				Expect(deployment.Spec.Strategy).To(Equal(v2.StrategyRollingUpdate))
			})

			It("should create the firewall set according to the deployment spec", func() {
				sets := &v2.FirewallSetList{}
				Eventually(func() []v2.FirewallSet {
					err := k8sClient.List(ctx, sets, client.InNamespace(deployment.Namespace))
					Expect(err).To(Not(HaveOccurred()))
					return sets.Items
				}, timeout, interval).Should(HaveLen(1))
				set := sets.Items[0]
				Expect(set.Name).To(HavePrefix(deployment.Name))
				Expect(set.Namespace).To(Equal(namespaceName))
				Expect(set.Spec.Replicas).To(Equal(1))
				wantSpec := deployment.Spec.Template.DeepCopy()
				wantSpec.Interval = "10s"
				Expect(&set.Spec.Template).To(BeComparableTo(wantSpec))
				Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
				Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment.Name))
			})

			It("should have the rbac condition succeeded", func() {
				deploy := deployment.DeepCopy()
				var cond *v2.Condition
				Eventually(func() *v2.Condition {
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deploy)
					Expect(err).To(Not(HaveOccurred()))
					cond = deploy.Status.Conditions.Get(v2.FirewallDeplomentRBACProvisioned)
					return cond
				}, timeout, interval).Should(Not(BeNil()))
				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Type).To(Equal(v2.FirewallDeplomentRBACProvisioned))
				Expect(cond.Status).To(Equal(v2.ConditionTrue))
				Expect(cond.Reason).To(Equal("Provisioned"))
				Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
			})
		})

		AfterAll(func() {
			By("cleaning up everyhing")

			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		})
	})
})
