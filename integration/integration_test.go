package controllers_test

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/metal-stack/metal-go/api/client/firewall"
	metalfirewall "github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
	metalclient "github.com/metal-stack/metal-go/test/client"
)

var (
	timeout  = 15 * time.Second
	interval = 250 * time.Millisecond
)

var _ = Describe("firewall deployment controller", Ordered, func() {
	Context("the good case", Ordered, func() {
		var (
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
				m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil)

				// we need to filter the orphan controller as it would delete the firewall
				call := m.On("FindFirewalls", mock.Anything, mock.Anything)
				call.Run(func(args mock.Arguments) {
					resp := &metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{}}
					params, ok := args.Get(0).(*firewall.FindFirewallsParams)
					if !ok {
						panic(fmt.Sprintf("unexpected type: %T", args.Get(0)))
					}
					if params.Body.AllocationName != "" {
						resp.Payload = append(resp.Payload, firewall1)
					}
					call.ReturnArguments = mock.Arguments{resp, nil}
				})
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
			var (
				fw  *v2.Firewall
				set *v2.FirewallSet
			)

			BeforeAll(func() {
				By("creating a firewall deployment")

				set = waitForSingleResource(&v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				})
				fw = waitForSingleResource(&v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
					return l.GetItems()
				})
			})

			Context("the firewall deployment resource", func() {
				It("should default the update strategy to rolling update (so the mutating webhook is working)", func() {
					Expect(deployment.Spec.Strategy).To(Equal(v2.StrategyRollingUpdate))
				})

				It("should have the rbac condition succeeded", func() {
					cond := waitForCondition(deployment.DeepCopy(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentRBACProvisioned)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Status).To(Equal(v2.ConditionTrue))
					Expect(cond.Reason).To(Equal("Provisioned"))
					Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
				})

				It("should have the rbac condition true", func() {
					cond := waitForCondition(deployment.DeepCopy(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentRBACProvisioned)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Status).To(Equal(v2.ConditionTrue))
					Expect(cond.Reason).To(Equal("Provisioned"))
					Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
				})

				It("should populate the status", func() {
					var deploy = deployment.DeepCopy()
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
						return set.Status.ReadyReplicas
					}, timeout, interval).Should(Equal(1), "reach ready replicas")

					Expect(deploy.Status.TargetReplicas).To(Equal(1))
					Expect(deploy.Status.ProgressingReplicas).To(Equal(0))
					Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
					Expect(deploy.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})

			Context("the firewall set resource", func() {
				It("should be named after the deployment", func() {
					Expect(set.Name).To(HavePrefix(deployment.Name + "-"))
				})

				It("should be in the same namespace as the deployment", func() {
					Expect(set.Namespace).To(Equal(deployment.Namespace))
				})

				It("should take the same replicas as defined by the deployement", func() {
					Expect(set.Spec.Replicas).To(Equal(1))
				})

				It("should inherit the spec from the deployement", func() {
					wantSpec := deployment.Spec.Template.DeepCopy()
					wantSpec.Interval = "10s"
					Expect(&set.Spec.Template).To(BeComparableTo(wantSpec))
				})

				It("should have the deployment as an owner", func() {
					Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment.Name))
				})

				It("should populate the status", func() {
					var set = set.DeepCopy()
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
						return set.Status.ReadyReplicas
					}, timeout, interval).Should(Equal(1), "reach ready replicas")

					Expect(set.Status.TargetReplicas).To(Equal(1))
					Expect(set.Status.ProgressingReplicas).To(Equal(0))
					Expect(set.Status.UnhealthyReplicas).To(Equal(0))
					Expect(set.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})

			Context("the firewall resource", func() {
				It("should be named after the namespace (it's the shoot name in the end)", func() {
					Expect(fw.Name).To(HavePrefix(namespaceName + "-firewall-"))
				})

				It("should be in the same namespace as the set", func() {
					Expect(fw.Namespace).To(Equal(set.Namespace))
				})

				It("should inherit the spec from the set", func() {
					wantSpec := set.Spec.Template.DeepCopy()
					wantSpec.Interval = "10s"
					Expect(&fw.Spec).To(BeComparableTo(wantSpec))
				})

				It("should have the deployment as an owner", func() {
					Expect(fw.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(fw.ObjectMeta.OwnerReferences[0].Name).To(Equal(set.Name))
				})

				It("should have the created condition true", func() {
					cond := waitForCondition(fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallCreated)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Status).To(Equal(v2.ConditionTrue))
					Expect(cond.Reason).To(Equal("Created"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q created successfully.", *firewall1.Allocation.Name)))
				})

				It("should populate the machine status", func() {
					var status *v2.MachineStatus
					var fw = fw.DeepCopy()
					Eventually(func() *v2.MachineStatus {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
						status = fw.Status.MachineStatus
						return status
					}, timeout, interval).Should(Not(BeNil()))

					Expect(status.MachineID).To(Equal(*firewall1.ID))
					Expect(status.CrashLoop).To(Equal(false))
					Expect(status.Liveliness).To(Equal("Alive"))
					Expect(status.LastEvent).NotTo(BeNil())
					Expect(status.LastEvent.Event).To(Equal("Phoned Home"))
					Expect(status.LastEvent.Message).To(Equal("phoning home"))
				})

				It("should have the ready condition true", func() {
					cond := waitForCondition(fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallReady)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Status).To(Equal(v2.ConditionTrue))
					Expect(cond.Reason).To(Equal("Ready"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is phoning home and alive.", *firewall1.Allocation.Name)))
				})

				It("should have the monitor condition true", func() {
					cond := waitForCondition(fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallMonitorDeployed)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Status).To(Equal(v2.ConditionTrue))
					Expect(cond.Reason).To(Equal("Deployed"))
					Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
				})
			})
		})

		AfterAll(func() {
			By("cleaning up everything")

			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())

			// TODO: check that all objects disappear
		})
	})
})

func waitForCondition[O client.Object](of O, getter func(O) v2.Conditions, t v2.ConditionType) *v2.Condition {
	var cond *v2.Condition
	Eventually(func() *v2.Condition {
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(of), of)).To(Succeed())
		cond = getter(of).Get(t)
		return cond
	}, timeout, interval).Should(Not(BeNil()))
	return cond
}

func waitForSingleResource[O client.Object, L client.ObjectList](list L, getter func(L) []O) O {
	var items []O
	Eventually(func() []O {
		err := k8sClient.List(ctx, list, client.InNamespace(namespaceName))
		Expect(err).To(Not(HaveOccurred()))
		items = getter(list)
		return items
	}, timeout, interval).Should(HaveLen(1))
	return items[0]
}
