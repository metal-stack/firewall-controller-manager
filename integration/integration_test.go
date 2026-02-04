package controllers_test

import (
	"fmt"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/metal-lib/httperrors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testcommon "github.com/metal-stack/firewall-controller-manager/integration/common"

	metalfirewall "github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/client/image"
	"github.com/metal-stack/metal-go/api/client/machine"
	"github.com/metal-stack/metal-go/api/client/network"
	"github.com/metal-stack/metal-go/api/models"
	metalclient "github.com/metal-stack/metal-go/test/client"
)

var (
	interval = 200 * time.Millisecond

	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}

	sshSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ssh-secret",
			Namespace: namespaceName,
		},
		StringData: map[string]string{
			"id_rsa":     "private",
			"id_rsa.pub": "public",
		},
	}

	genericKubeconfigSecret = func(apiCA, apiHost, apiCert, apiKey string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubeconfig-secret-name",
				Namespace: namespaceName,
			},
			Data: map[string][]byte{
				"kubeconfig": []byte(fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: shoot-name
contexts:
- context:
    cluster: shoot-name
    user: shoot-name
  name: shoot-name
current-context: shoot-name
kind: Config
preferences: {}
users:
- name: shoot-name
  user:
    client-certificate-data: %s
    client-key-data: %s

`, apiCA, apiHost, apiCert, apiKey))},
		}
	}

	shootTokenSecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "token",
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			"token": []byte(`eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImlhdCI6MTUxNjIzOTAyMn0.NHVaYe26MbtOYhSKkoKYdFVomg4i8ZJd8_-RU8VNbftc4TSMb4bXP3l3YlNWACwyXPGffz5aXHc6lty1Y2t4SWRqGteragsVdZufDn5BlnJl9pdR_kdVFUsra2rWKEofkZeIC4yWytE58sMIihvo9H1ScmmVwBcQP6XETqYd0aSHp1gOa9RdUPDvoXQ5oqygTqVtxaDr6wUFKrKItgBMzWIdNZ6y7O9E0DhEPTbE9rfBo6KTFsHAZnMg4k68CDp2woYIaXbmYTWcvbzIuHO7_37GT79XdIwkm95QJ7hYC9RiwrV7mesbY4PAahERJawntho0my942XheVLmGwLMBkQ`),
		},
	}

	// we need to fake the secret as there is no kube-controller-manager in the
	// envtest setup which can issue a long-lived token for the secret
	fakeTokenSecretSeed = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-seed-access-test",
			Namespace: namespaceName,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": "firewall-controller-seed-access-test",
			},
		},
		StringData: map[string]string{
			"token":  "a-token",
			"ca.crt": "ca-crt",
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	fakeTokenSecretShoot = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "firewall-controller-shoot-access-test",
			Namespace: namespaceName,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": "firewall-controller-shoot-access-test",
			},
		},
		StringData: map[string]string{
			"token":  "a-token",
			"ca.crt": "ca-crt",
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
)

var _ = Context("integration test", Ordered, func() {
	var (
		deployment = func() *v2.FirewallDeployment {
			return &v2.FirewallDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: namespaceName,
				},
				Spec: v2.FirewallDeploymentSpec{
					Replicas: 1,
					Template: v2.FirewallTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"purpose": "shoot-firewall",
							},
						},
						Spec: v2.FirewallSpec{
							Size:                    "n1-medium-x86",
							Project:                 "project-a",
							Partition:               "partition-a",
							Image:                   "firewall-ubuntu-2.0",
							Networks:                []string{"internet"},
							ControllerURL:           "http://controller.tar.gz",
							ControllerVersion:       "v2.0.0",
							NftablesExporterURL:     "http://exporter.tar.gz",
							NftablesExporterVersion: "v1.0.0",
							Interval:                defaults.DefaultFirewallReconcileInterval,
						},
					},
				},
			}
		}
	)

	BeforeAll(func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, namespace.DeepCopy()))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, fakeTokenSecretSeed.DeepCopy()))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, fakeTokenSecretShoot.DeepCopy()))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, sshSecret.DeepCopy()))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, genericKubeconfigSecret(apiCA, apiHost, apiCert, apiKey)))).To(Succeed())
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, shootTokenSecret.DeepCopy()))).To(Succeed())
	})

	Describe("the rolling update", Ordered, func() {
		When("creating a firewall deployment", Ordered, func() {
			It("the creation works", func() {
				swapMetalClient(&metalclient.MetalMockFns{
					Firewall: func(m *mock.Mock) {
						m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil).Maybe()
						m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
						m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
					},
					Network: func(m *mock.Mock) {
						m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
					},
					Machine: func(m *mock.Mock) {
						m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
					},
					Image: func(m *mock.Mock) {
						m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
					},
				})

				Expect(k8sClient.Create(ctx, deployment())).To(Succeed())
			})

			It("the userdata was rendered by the defaulting webhook", func() {
				deploy := &v2.FirewallDeployment{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
				Expect(deploy.Spec.Template.Spec.Userdata).NotTo(BeEmpty())
			})

			It("the update strategy is rolling update", func() {
				deploy := &v2.FirewallDeployment{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
				Expect(deploy.Spec.Strategy).To(Equal(v2.StrategyRollingUpdate))
			})
		})

		Describe("new resources will be spawned by the controller", Ordered, func() {
			var (
				fw  *v2.Firewall
				set *v2.FirewallSet
				mon *v2.FirewallMonitor
			)

			It("should create a firewall set", func() {
				set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should create a firewall", func() {
				fw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
					return l.GetItems()
				}, 15*time.Second)

				// Prevent immediate health-timeout in tests by setting a recent seed reconciliation time.
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
				if fw.Status.ControllerStatus == nil {
					fw.Status.ControllerStatus = &v2.ControllerConnection{}
				}
				fw.Status.ControllerStatus.SeedUpdated = metav1.Now()
				Expect(k8sClient.Status().Update(ctx, fw)).To(Succeed())
			})

			It("should create a firewall monitor", func() {
				mon = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should allow an update of the firewall monitor", func() {
				// simulating a firewall-controller updating the resource
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mon), mon)).To(Succeed()) // refetch
				mon.ControllerStatus = &v2.ControllerStatus{
					Updated:           metav1.NewTime(time.Now()),
					SeedUpdated:       metav1.NewTime(time.Now()),
					Distance:          v2.FirewallShortestDistance,
					DistanceSupported: true,
				}
				Expect(k8sClient.Update(ctx, mon)).To(Succeed())
			})

			Context("the firewall resource", func() {
				It("should be named after the namespace (it's the shoot name in the end)", func() {
					Expect(fw.Name).To(HavePrefix(namespaceName + "-firewall-"))
				})

				It("should be in the same namespace as the set", func() {
					Expect(fw.Namespace).To(Equal(set.Namespace))
				})

				It("should inherit the spec from the set", func() {
					wantSpec := set.Spec.Template.Spec.DeepCopy()
					Expect(&fw.Spec).To(BeComparableTo(wantSpec))
				})

				It("should have the set as an owner", func() {
					Expect(fw.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(fw.ObjectMeta.OwnerReferences[0].Name).To(Equal(set.Name))
				})

				It("should have the created condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallCreated, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
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
					}, 5*time.Second, interval).Should(Not(BeNil()))

					Expect(status.MachineID).To(Equal(*firewall1.ID))
					Expect(status.CrashLoop).To(Equal(false))
					Expect(status.Liveliness).To(Equal("Alive"))
					Expect(status.LastEvent).NotTo(BeNil())
					Expect(status.LastEvent.Event).To(Equal("Phoned Home"))
					Expect(status.LastEvent.Message).To(Equal("phoning home"))
				})

				It("should have the ready condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallReady, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Ready"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is phoning home and alive.", *firewall1.Allocation.Name)))
				})

				It("should have the monitor condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallMonitorDeployed, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Deployed"))
					Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
				})

				It("should have the firewall-controller connected condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallControllerConnected, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Connected"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled shoot at %s.", mon.ControllerStatus.Updated.String())))
				})
				It("should have the firewall-controller connected to seed condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallControllerSeedConnected, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Connected"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled firewall at %s.", mon.ControllerStatus.SeedUpdated.String())))
				})
				It("should have configured the distance", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDistanceConfigured, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Configured"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller has configured the specified distance %d.", v2.FirewallShortestDistance)))
				})
			})

			Context("the firewall set resource", func() {
				It("should be named after the deployment", func() {
					Expect(set.Name).To(HavePrefix(deployment().Name + "-"))
				})

				It("should be in the same namespace as the deployment", func() {
					Expect(set.Namespace).To(Equal(deployment().Namespace))
				})

				It("should take the same replicas as defined by the deployment", func() {
					Expect(set.Spec.Replicas).To(Equal(1))
				})

				It("should inherit the spec from the deployment", func() {
					deploy := &v2.FirewallDeployment{}
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
					Expect(set.Spec.Template.Spec).To(BeComparableTo(deploy.Spec.Template.Spec))
				})

				It("should have the deployment as an owner", func() {
					Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment().Name))
				})

				It("should populate the status", func() {
					var set = set.DeepCopy()
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
						return set.Status.ReadyReplicas
					}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

					Expect(set.Status.TargetReplicas).To(Equal(1))
					Expect(set.Status.ProgressingReplicas).To(Equal(0))
					Expect(set.Status.UnhealthyReplicas).To(Equal(0))
					Expect(set.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})

			Context("the firewall deployment resource", func() {
				It("should default the update strategy to rolling update (so the mutating webhook is working)", func() {
					deploy := &v2.FirewallDeployment{}
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
					Expect(deploy.Spec.Strategy).To(Equal(v2.StrategyRollingUpdate))
				})

				It("should have the rbac condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Provisioned"))
					Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
				})

				It("should have the available condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentAvailable, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("MinimumReplicasAvailable"))
					Expect(cond.Message).To(Equal("Deployment has minimum availability."))
				})

				It("should have the progress condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentProgressing, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Or(Equal("NewFirewallSetAvailable"), Equal("FirewallSetUpdated")))
					Expect(cond.Message).To(Or(
						Equal(fmt.Sprintf("FirewallSet %q has successfully progressed.", set.Name)),
						Equal(fmt.Sprintf("Updated firewall set %q.", set.Name)),
					))
				})

				It("should populate the status", func() {
					deploy := &v2.FirewallDeployment{}
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
						return deploy.Status.ReadyReplicas
					}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

					Expect(deploy.Status.TargetReplicas).To(Equal(1))
					Expect(deploy.Status.ProgressingReplicas).To(Equal(0))
					Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
					Expect(deploy.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})
		})

		When("a significant change occurs", Ordered, func() {
			var (
				installingFirewall = firewall2("Installing", "is installing")
			)

			Context("the spec is updated", func() {
				It("the update works", func() {
					swapMetalClient(&metalclient.MetalMockFns{
						Firewall: func(m *mock.Mock) {
							m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: installingFirewall}, nil).Maybe()
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: installingFirewall}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{installingFirewall}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Machine: func(m *mock.Mock) {
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					deploy := deployment()
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())

					deploy.Spec.Template.Spec.Size = "n2-medium-x86"

					Expect(k8sClient.Update(ctx, deploy)).To(Succeed())
				})
			})

			var (
				fw  *v2.Firewall
				set *v2.FirewallSet
				mon *v2.FirewallMonitor
			)

			Context("new resources will be spawned by the controller", func() {

				It("should create another firewall set", func() {
					set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 2, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
						return l.GetItems()
					}, 15*time.Second)
				})

				It("should create another firewall", func() {
					fw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 2, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
						return l.GetItems()
					}, 15*time.Second)
				})

				It("should create another firewall monitor", func() {
					mon = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 2, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
						return l.GetItems()
					}, 15*time.Second)
				})

				Context("the new firewall resource", func() {
					It("should be named after the namespace (it's the shoot name in the end)", func() {
						Expect(fw.Name).To(HavePrefix(namespaceName + "-firewall-"))
					})

					It("should be in the same namespace as the set", func() {
						Expect(fw.Namespace).To(Equal(set.Namespace))
					})

					It("should inherit the spec from the set", func() {
						wantSpec := set.Spec.Template.Spec.DeepCopy()
						Expect(&fw.Spec).To(BeComparableTo(wantSpec))
					})

					It("should have the set as an owner", func() {
						Expect(fw.ObjectMeta.OwnerReferences).To(HaveLen(1))
						Expect(fw.ObjectMeta.OwnerReferences[0].Name).To(Equal(set.Name))
					})

					It("should have the created condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallCreated, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Created"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q created successfully.", *installingFirewall.Allocation.Name)))
					})

					It("should populate the machine status", func() {
						var status *v2.MachineStatus
						var fw = fw.DeepCopy()
						Eventually(func() *v2.MachineStatus {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							status = fw.Status.MachineStatus
							return status
						}, 5*time.Second, interval).Should(Not(BeNil()))

						Expect(status.MachineID).To(Equal(*installingFirewall.ID))
						Expect(status.CrashLoop).To(Equal(false))
						Expect(status.Liveliness).To(Equal("Alive"))
						Expect(status.LastEvent).NotTo(BeNil())
						Expect(status.LastEvent.Event).To(Equal("Installing"))
						Expect(status.LastEvent.Message).To(Equal("is installing"))
					})

					It("should have the ready condition false", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallReady, v2.ConditionFalse, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("NotReady"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is not ready.", *installingFirewall.Allocation.Name)))
					})

					It("should not yet have a distance configured", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDistanceConfigured, v2.ConditionFalse, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("NotConnected"))
						Expect(cond.Message).To(Equal("Controller has not yet connected."))
					})

					It("should have the monitor condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallMonitorDeployed, v2.ConditionTrue, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Deployed"))
						Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
					})

					It("should have the firewall-controller connected condition false", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallControllerConnected, v2.ConditionFalse, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
					})

					It("should have firewall networks populated", func() {
						var nws []v2.FirewallNetwork
						var fw = fw.DeepCopy()
						Eventually(func() []v2.FirewallNetwork {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							nws = fw.Status.FirewallNetworks
							return nws
						}, 5*time.Second, interval).Should(HaveLen(1))

						Expect(nws).To(BeComparableTo([]v2.FirewallNetwork{
							{
								ASN:                 installingFirewall.Allocation.Networks[0].Asn,
								DestinationPrefixes: installingFirewall.Allocation.Networks[0].Destinationprefixes,
								IPs:                 installingFirewall.Allocation.Networks[0].Ips,
								Nat:                 installingFirewall.Allocation.Networks[0].Nat,
								NetworkID:           installingFirewall.Allocation.Networks[0].Networkid,
								NetworkType:         installingFirewall.Allocation.Networks[0].Networktype,
								Prefixes:            network1.Prefixes,
								Vrf:                 installingFirewall.Allocation.Networks[0].Vrf,
							},
						}))
					})

					It("should have shoot access populated", func() {
						var access *v2.ShootAccess
						var fw = fw.DeepCopy()
						Eventually(func() *v2.ShootAccess {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							access = fw.Status.ShootAccess
							return access
						}, 5*time.Second, interval).Should(Not(BeNil()))

						Expect(access).To(BeComparableTo(&v2.ShootAccess{
							GenericKubeconfigSecretName: "kubeconfig-secret-name",
							TokenSecretName:             "token",
							Namespace:                   namespaceName,
							APIServerURL:                apiHost,
						}))
					})
				})

				Context("the new firewall set resource", func() {
					It("should be named after the deployment", func() {
						Expect(set.Name).To(HavePrefix(deployment().Name + "-"))
					})

					It("should be in the same namespace as the deployment", func() {
						Expect(set.Namespace).To(Equal(deployment().Namespace))
					})

					It("should take the same replicas as defined by the deployment", func() {
						Expect(set.Spec.Replicas).To(Equal(1))
					})

					It("should inherit the spec from the deployment", func() {
						deploy := &v2.FirewallDeployment{}
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())

						wantSpec := deploy.Spec.Template.Spec.DeepCopy()
						wantSpec.Size = "n2-medium-x86" // this is the change that triggered the rolling update
						Expect(&set.Spec.Template.Spec).To(BeComparableTo(wantSpec))
					})

					It("should start with a higher distance", func() {
						Expect(set.Spec.Distance).To(Equal(v2.FirewallRollingUpdateSetDistance))
					})

					It("should have the deployment as an owner", func() {
						Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
						Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment().Name))
					})

					It("should populate the status", func() {
						var set = set.DeepCopy()
						Eventually(func() int {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
							return set.Status.ProgressingReplicas
						}, 15*time.Second, interval).Should(Equal(1), "reach progressing replicas")

						Expect(set.Status.TargetReplicas).To(Equal(1))
						Expect(set.Status.ReadyReplicas).To(Equal(0))
						Expect(set.Status.UnhealthyReplicas).To(Equal(0))
						Expect(set.Status.ObservedRevision).To(Equal(1))
					})
				})

				Context("the firewall deployment resource", func() {
					It("should have the rbac condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Provisioned"))
						Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
					})

					It("should have the available condition false", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentAvailable, v2.ConditionFalse, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("MinimumReplicasUnavailable"))
						Expect(cond.Message).To(Equal("Deployment does not have minimum availability."))
					})

					It("should have the progress condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentProgressing, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Or(Equal("NewFirewallSetAvailable"), Equal("FirewallSetUpdated")))
						Expect(cond.Message).To(Or(
							Equal(fmt.Sprintf("FirewallSet %q has successfully progressed.", set.Name)),
							Equal(fmt.Sprintf("Updated firewall set %q.", set.Name)),
						))
					})

					It("should populate the status", func() {
						deploy := &v2.FirewallDeployment{}
						Eventually(func() int {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
							return deploy.Status.ProgressingReplicas
						}, 15*time.Second, interval).Should(Equal(1), "reach progressing replicas")

						Expect(deploy.Status.TargetReplicas).To(Equal(1))
						Expect(deploy.Status.ReadyReplicas).To(Equal(0))
						Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
						Expect(deploy.Status.ObservedRevision).To(Equal(1))
					})

					It("should not be possible to update deployment strategy while deployment has not converged", func() {
						deploy := &v2.FirewallDeployment{}
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
						deploy.Spec.Strategy = v2.StrategyRecreate
						err := k8sClient.Update(ctx, deploy)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring(`Invalid value: "Recreate": strategy can not be updated until target replicas have been reached (i.e. deployment has converged)`))
					})
				})

				// TODO: verify the monitor resource
			})

			var (
				readyFirewall = firewall2("Phoned Home", "is phoning home")
			)

			When("the firewall gets ready and the firewall-controller connects", Ordered, func() {
				It("should allow an update of the firewall monitor", func() {
					swapMetalClient(&metalclient.MetalMockFns{
						Machine: func(m *mock.Mock) {
							m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Firewall: func(m *mock.Mock) {
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: readyFirewall}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{readyFirewall}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mon), mon)).To(Succeed()) // refetch
					// simulating a firewall-controller updating the resource
					mon.ControllerStatus = &v2.ControllerStatus{
						Updated:           metav1.NewTime(time.Now()),
						SeedUpdated:       metav1.NewTime(time.Now()),
						Distance:          v2.FirewallRollingUpdateSetDistance,
						DistanceSupported: true,
					}
					Expect(k8sClient.Update(ctx, mon)).To(Succeed())
				})

				It("the firewall-controller reflects the distance during a rolling update", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDistanceConfigured, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Configured"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller has configured the specified distance %d.", v2.FirewallRollingUpdateSetDistance)))
				})

				Context("the old generation disappears", Ordered, func() {
					var (
						fw  *v2.Firewall
						set *v2.FirewallSet
					)

					It("should delete firewall monitor", func() {
						mon := testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
							return l.GetItems()
						}, 15*time.Second)
						Expect(mon.MachineStatus.MachineID).To(Equal(*readyFirewall.ID))
					})

					It("should delete the firewall", func() {
						fw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
							return l.GetItems()
						}, 15*time.Second)
						Expect(fw.Status.MachineStatus.MachineID).To(Equal(*readyFirewall.ID))
					})

					It("should delete the firewall set", func() {
						set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
							return l.GetItems()
						}, 15*time.Second)
						Expect(set.Status.ObservedRevision).To(Equal(1))
					})

					Context("the update is finalized", func() {
						It("should populate the controller status field in the firewall resource", func() {
							var fw = fw.DeepCopy()
							Eventually(func() *v2.ControllerConnection {
								Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
								return fw.Status.ControllerStatus
							}, 15*time.Second, interval).Should(Not(BeNil()), "controller connection was not synced to firewall resource")
						})

						It("the firewall set should be updated to shortest distance as the update has succeeded", func() {
							Eventually(func() v2.FirewallDistance {
								Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
								return set.Spec.Distance
							}).Within(5 * time.Second).ProbeEvery(interval).Should(Equal(v2.FirewallShortestDistance))
							Expect(set.Spec.Distance).To(Equal(v2.FirewallShortestDistance))
						})
					})
				})
			})
		})

		Describe("the deletion flow", Ordered, func() {
			When("deleting the firewall deployment", func() {
				It("the deletion finishes", func() {
					swapMetalClient(&metalclient.MetalMockFns{
						Firewall: func(m *mock.Mock) {
							m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Machine: func(m *mock.Mock) {
							m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					Expect(k8sClient.Delete(ctx, deployment())).To(Succeed())

					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallDeploymentList{}, func(l *v2.FirewallDeploymentList) []*v2.FirewallDeployment {
						return l.GetItems()
					}, 10*time.Second)
				})
			})

			Context("all resources are cleaned up", func() {
				It("should delete the firewall set", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete the firewall", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete firewall monitor", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
						return l.GetItems()
					}, 10*time.Second)
				})
			})
		})
	})

	Describe("the recreate update", Ordered, func() {
		When("creating a firewall deployment", Ordered, func() {
			It("the creation works", func() {
				swapMetalClient(&metalclient.MetalMockFns{
					Firewall: func(m *mock.Mock) {
						m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil).Maybe()
						m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
						m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
					},
					Network: func(m *mock.Mock) {
						m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
					},
					Machine: func(m *mock.Mock) {
						m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
					},
					Image: func(m *mock.Mock) {
						m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
					},
				})

				deploy := deployment()
				deploy.Spec.Strategy = v2.StrategyRecreate
				Expect(k8sClient.Create(ctx, deploy)).To(Succeed())
			})

			It("the userdata was rendered by the defaulting webhook", func() {
				deploy := &v2.FirewallDeployment{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
				Expect(deploy.Spec.Template.Spec.Userdata).NotTo(BeEmpty())
			})

			It("the update strategy is recreate", func() {
				deploy := &v2.FirewallDeployment{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
				Expect(deploy.Spec.Strategy).To(Equal(v2.StrategyRecreate))
			})
		})

		Describe("new resources will be spawned by the controller", Ordered, func() {
			var (
				fw  *v2.Firewall
				set *v2.FirewallSet
				mon *v2.FirewallMonitor
			)

			It("should create a firewall set", func() {
				set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should create a firewall", func() {
				fw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should create a firewall monitor", func() {
				mon = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should allow an update of the firewall monitor", func() {
				// simulating a firewall-controller updating the resource
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mon), mon)).To(Succeed()) // refetch
				mon.ControllerStatus = &v2.ControllerStatus{
					Updated:           metav1.NewTime(time.Now()),
					SeedUpdated:       metav1.NewTime(time.Now()),
					Distance:          v2.FirewallShortestDistance,
					DistanceSupported: true,
				}
				Expect(k8sClient.Update(ctx, mon)).To(Succeed())
			})

			Context("the firewall resource", func() {
				It("should be named after the namespace (it's the shoot name in the end)", func() {
					Expect(fw.Name).To(HavePrefix(namespaceName + "-firewall-"))
				})

				It("should be in the same namespace as the set", func() {
					Expect(fw.Namespace).To(Equal(set.Namespace))
				})

				It("should inherit the spec from the set", func() {
					wantSpec := set.Spec.Template.Spec.DeepCopy()
					Expect(&fw.Spec).To(BeComparableTo(wantSpec))
				})

				It("should have the set as an owner", func() {
					Expect(fw.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(fw.ObjectMeta.OwnerReferences[0].Name).To(Equal(set.Name))
				})

				It("should have the created condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallCreated, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
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
					}, 5*time.Second, interval).Should(Not(BeNil()))

					Expect(status.MachineID).To(Equal(*firewall1.ID))
					Expect(status.CrashLoop).To(Equal(false))
					Expect(status.Liveliness).To(Equal("Alive"))
					Expect(status.LastEvent).NotTo(BeNil())
					Expect(status.LastEvent.Event).To(Equal("Phoned Home"))
					Expect(status.LastEvent.Message).To(Equal("phoning home"))
				})

				It("should have the ready condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallReady, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Ready"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is phoning home and alive.", *firewall1.Allocation.Name)))
				})

				It("should have the monitor condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallMonitorDeployed, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Deployed"))
					Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
				})

				It("should have the firewall-controller connected condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallControllerConnected, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Connected"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled shoot at %s.", mon.ControllerStatus.Updated.String())))
				})
				It("should have the firewall-controller connected to seed condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallControllerSeedConnected, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Connected"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled firewall at %s.", mon.ControllerStatus.SeedUpdated.String())))
				})
				It("should have configured the distance", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDistanceConfigured, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Configured"))
					Expect(cond.Message).To(Equal(fmt.Sprintf("Controller has configured the specified distance %d.", v2.FirewallShortestDistance)))
				})
			})

			Context("the firewall set resource", func() {
				It("should be named after the deployment", func() {
					Expect(set.Name).To(HavePrefix(deployment().Name + "-"))
				})

				It("should be in the same namespace as the deployment", func() {
					Expect(set.Namespace).To(Equal(deployment().Namespace))
				})

				It("should take the same replicas as defined by the deployment", func() {
					Expect(set.Spec.Replicas).To(Equal(1))
				})

				It("should inherit the spec from the deployment", func() {
					deploy := &v2.FirewallDeployment{}
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
					Expect(set.Spec.Template.Spec).To(BeComparableTo(deploy.Spec.Template.Spec))
				})

				It("should have the deployment as an owner", func() {
					Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
					Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment().Name))
				})

				It("should populate the status", func() {
					var set = set.DeepCopy()
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
						return set.Status.ReadyReplicas
					}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

					Expect(set.Status.TargetReplicas).To(Equal(1))
					Expect(set.Status.ProgressingReplicas).To(Equal(0))
					Expect(set.Status.UnhealthyReplicas).To(Equal(0))
					Expect(set.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})

			Context("the firewall deployment resource", func() {
				It("should have the update strategy recreate", func() {
					deploy := &v2.FirewallDeployment{}
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
					Expect(deploy.Spec.Strategy).To(Equal(v2.StrategyRecreate))
				})

				It("should have the rbac condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("Provisioned"))
					Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
				})

				It("should have the available condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentAvailable, v2.ConditionTrue, 5*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Equal("MinimumReplicasAvailable"))
					Expect(cond.Message).To(Equal("Deployment has minimum availability."))
				})

				It("should have the progress condition true", func() {
					cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
						return fd.Status.Conditions
					}, v2.FirewallDeplomentProgressing, v2.ConditionTrue, 15*time.Second)

					Expect(cond.LastTransitionTime).NotTo(BeZero())
					Expect(cond.LastUpdateTime).NotTo(BeZero())
					Expect(cond.Reason).To(Or(Equal("NewFirewallSetAvailable"), Equal("FirewallSetUpdated")))
					Expect(cond.Message).To(Or(
						Equal(fmt.Sprintf("FirewallSet %q has successfully progressed.", set.Name)),
						Equal(fmt.Sprintf("Updated firewall set %q.", set.Name)),
					))
				})

				It("should populate the status", func() {
					deploy := &v2.FirewallDeployment{}
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
						return deploy.Status.ReadyReplicas
					}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

					Expect(deploy.Status.TargetReplicas).To(Equal(1))
					Expect(deploy.Status.ProgressingReplicas).To(Equal(0))
					Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
					Expect(deploy.Status.ObservedRevision).To(Equal(0)) // this is the first revision
				})
			})

			// TODO: verify monitor resource
		})

		When("a significant change occurs", Ordered, func() {
			var (
				readyFirewall = firewall2("Phoned Home", "is phoning home")

				oldFw  *v2.Firewall
				oldSet *v2.FirewallSet
				oldMon *v2.FirewallMonitor
			)

			It("should find a single firewall set before the update", func() {
				oldSet = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				}, 5*time.Second)
			})

			It("should find a single firewall before the update", func() {
				oldFw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
					return l.GetItems()
				}, 5*time.Second)
			})

			It("should find a single firewall monitor before the update", func() {
				oldMon = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
					return l.GetItems()
				}, 5*time.Second)
			})

			Context("the spec is updated", Ordered, func() {
				It("the update works", func() {
					deploy := deployment()

					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())

					swapMetalClient(&metalclient.MetalMockFns{
						Firewall: func(m *mock.Mock) {
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: readyFirewall}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{readyFirewall}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Machine: func(m *mock.Mock) {
							m.On("FreeMachine", mock.Anything, nil).Return(nil, &machine.FreeMachineDefault{Payload: httperrors.Conflict(fmt.Errorf("deletion blocked"))}).Maybe()
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					deploy.Spec.Template.Spec.Networks = []string{"internet", "mpls"}

					Expect(k8sClient.Update(ctx, deploy)).To(Succeed())
				})

				It("a new generation appears with 0 replicas", func() {
					set := &v2.FirewallSet{}
					Eventually(func() int {
						set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 2, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
							return l.GetItems()
						}, 15*time.Second)
						return set.Spec.Replicas
					}, 15*time.Second, interval).Should(Equal(0))
					Eventually(func() int {
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
						return set.Status.ObservedRevision // revision is set after first status update, before that it's 0
					}, 3*time.Second, interval).Should(Equal(1))
				})

				Context("the old generation disappears", Ordered, func() {
					It("should delete the firewall set", func() {
						swapMetalClient(&metalclient.MetalMockFns{
							Firewall: func(m *mock.Mock) {
								m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: readyFirewall}, nil).Maybe()
								m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: readyFirewall}, nil).Maybe()
								m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{readyFirewall}}, nil).Maybe()
							},
							Network: func(m *mock.Mock) {
								m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
							},
							Machine: func(m *mock.Mock) {
								m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
								m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
							},
							Image: func(m *mock.Mock) {
								m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
							},
						})

						Eventually(func() bool {
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(oldSet), oldSet)
							return apierrors.IsNotFound(err)
						}, 15*time.Second, interval).Should(BeTrue())
					})

					It("should delete the firewall", func() {
						Eventually(func() bool {
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(oldFw), oldFw)
							return apierrors.IsNotFound(err)
						}, 15*time.Second, interval).Should(BeTrue())
					})

					It("should delete firewall monitor", func() {
						Eventually(func() bool {
							err := k8sClient.Get(ctx, client.ObjectKeyFromObject(oldMon), oldMon)
							return apierrors.IsNotFound(err)
						}, 15*time.Second, interval).Should(BeTrue())
					})
				})

				var (
					newFw  *v2.Firewall
					newSet *v2.FirewallSet
					newMon *v2.FirewallMonitor
				)

				Context("a new generation is scaled up", func() {
					It("should scale up the new set", func() {
						Eventually(func() int {
							newSet = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
								return l.GetItems()
							}, 15*time.Second)
							return newSet.Spec.Replicas
						}, 15*time.Second, interval).Should(Equal(1))
					})

					It("should create a new firewall", func() {
						newFw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
							return l.GetItems()
						}, 15*time.Second)
					})

					It("should create a firewall monitor", func() {
						newMon = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
							return l.GetItems()
						}, 15*time.Second)
					})
				})

				When("the firewall-controller connects", Ordered, func() {
					It("should allow an update of the firewall monitor", func() {
						swapMetalClient(&metalclient.MetalMockFns{
							Machine: func(m *mock.Mock) {
								m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
							},
							Firewall: func(m *mock.Mock) {
								m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: readyFirewall}, nil).Maybe()
								m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{readyFirewall}}, nil).Maybe()
							},
							Network: func(m *mock.Mock) {
								m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
							},
							Image: func(m *mock.Mock) {
								m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
							},
						})

						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(newMon), newMon)).To(Succeed()) // refetch
						// simulating a firewall-controller updating the resource
						newMon.ControllerStatus = &v2.ControllerStatus{
							Updated:           metav1.NewTime(time.Now()),
							SeedUpdated:       metav1.NewTime(time.Now()),
							Distance:          v2.FirewallShortestDistance,
							DistanceSupported: true,
						}
						Expect(k8sClient.Update(ctx, newMon)).To(Succeed())
					})
				})

				Context("the new firewall resource", func() {
					It("should be named after the namespace (it's the shoot name in the end)", func() {
						Expect(newFw.Name).To(HavePrefix(namespaceName + "-firewall-"))
					})

					It("should be in the same namespace as the set", func() {
						Expect(newFw.Namespace).To(Equal(newSet.Namespace))
					})

					It("should inherit the spec from the set", func() {
						wantSpec := newSet.Spec.Template.Spec.DeepCopy()
						Expect(&newFw.Spec).To(BeComparableTo(wantSpec))
					})

					It("should have the set as an owner", func() {
						Expect(newFw.ObjectMeta.OwnerReferences).To(HaveLen(1))
						Expect(newFw.ObjectMeta.OwnerReferences[0].Name).To(Equal(newSet.Name))
					})

					It("should have the created condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, newFw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallCreated, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Created"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q created successfully.", *readyFirewall.Allocation.Name)))
					})

					It("should populate the machine status", func() {
						var status *v2.MachineStatus
						var fw = newFw.DeepCopy()
						Eventually(func() *v2.MachineStatus {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							status = fw.Status.MachineStatus
							return status
						}, 5*time.Second, interval).Should(Not(BeNil()))

						Expect(status.MachineID).To(Equal(*readyFirewall.ID))
						Expect(status.CrashLoop).To(Equal(false))
						Expect(status.Liveliness).To(Equal("Alive"))
						Expect(status.LastEvent).NotTo(BeNil())
						Expect(status.LastEvent.Event).To(Equal("Phoned Home"))
						Expect(status.LastEvent.Message).To(Equal("is phoning home"))
					})

					It("should have the ready condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, newFw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallReady, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Ready"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is phoning home and alive.", *readyFirewall.Allocation.Name)))
					})

					It("should have the monitor condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, newFw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallMonitorDeployed, v2.ConditionTrue, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Deployed"))
						Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
					})

					It("should have the firewall-controller connected condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, newFw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallControllerConnected, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Connected"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled shoot at %s.", newMon.ControllerStatus.Updated.String())))
					})

					It("should have the firewall-controller connected to seed condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, newFw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallControllerSeedConnected, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Connected"))
						Expect(cond.Message).To(Equal(fmt.Sprintf("Controller reconciled firewall at %s.", newMon.ControllerStatus.SeedUpdated.String())))
					})

					It("should have firewall networks populated", func() {
						var nws []v2.FirewallNetwork
						var fw = newFw.DeepCopy()
						Eventually(func() []v2.FirewallNetwork {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							nws = fw.Status.FirewallNetworks
							return nws
						}, 5*time.Second, interval).Should(HaveLen(1))

						Expect(nws).To(BeComparableTo([]v2.FirewallNetwork{
							{
								ASN:                 readyFirewall.Allocation.Networks[0].Asn,
								DestinationPrefixes: readyFirewall.Allocation.Networks[0].Destinationprefixes,
								IPs:                 readyFirewall.Allocation.Networks[0].Ips,
								Nat:                 readyFirewall.Allocation.Networks[0].Nat,
								NetworkID:           readyFirewall.Allocation.Networks[0].Networkid,
								NetworkType:         readyFirewall.Allocation.Networks[0].Networktype,
								Prefixes:            network1.Prefixes,
								Vrf:                 readyFirewall.Allocation.Networks[0].Vrf,
							},
						}))
					})

					It("should have shoot access populated", func() {
						var access *v2.ShootAccess
						var fw = newFw.DeepCopy()
						Eventually(func() *v2.ShootAccess {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
							access = fw.Status.ShootAccess
							return access
						}, 5*time.Second, interval).Should(Not(BeNil()))

						Expect(access).To(BeComparableTo(&v2.ShootAccess{
							GenericKubeconfigSecretName: "kubeconfig-secret-name",
							TokenSecretName:             "token",
							Namespace:                   namespaceName,
							APIServerURL:                apiHost,
						}))
					})

					It("should have the shortest distance", func() {
						Expect(newFw.Distance).To(Equal(v2.FirewallShortestDistance))
					})
				})

				Context("the new firewall set resource", func() {
					It("should be named after the deployment", func() {
						Expect(newSet.Name).To(HavePrefix(deployment().Name + "-"))
					})

					It("should be in the same namespace as the deployment", func() {
						Expect(newSet.Namespace).To(Equal(deployment().Namespace))
					})

					It("should inherit the spec from the deployment", func() {
						deploy := &v2.FirewallDeployment{}
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
						wantSpec := deploy.Spec.Template.Spec.DeepCopy()
						wantSpec.Networks = []string{"internet", "mpls"} // this is the change that triggered the recreate update
						Expect(&newSet.Spec.Template.Spec).To(BeComparableTo(wantSpec))
					})

					It("should have the shortest distance", func() {
						Expect(newSet.Spec.Distance).To(Equal(v2.FirewallShortestDistance))
					})

					It("should have the deployment as an owner", func() {
						Expect(newSet.ObjectMeta.OwnerReferences).To(HaveLen(1))
						Expect(newSet.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment().Name))
					})

					It("should populate the status", func() {
						var set = newSet.DeepCopy()
						Eventually(func() int {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
							return set.Status.ReadyReplicas
						}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

						Expect(set.Status.TargetReplicas).To(Equal(1))
						Expect(set.Status.ProgressingReplicas).To(Equal(0))
						Expect(set.Status.UnhealthyReplicas).To(Equal(0))
						Expect(set.Status.ObservedRevision).To(Equal(1))
					})
				})

				Context("the firewall deployment resource", func() {
					It("should have the rbac condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("Provisioned"))
						Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
					})

					It("should have the available condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentAvailable, v2.ConditionTrue, 5*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Equal("MinimumReplicasAvailable"))
						Expect(cond.Message).To(Equal("Deployment has minimum availability."))
					})

					It("should have the progress condition true", func() {
						cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
							return fd.Status.Conditions
						}, v2.FirewallDeplomentProgressing, v2.ConditionTrue, 15*time.Second)

						Expect(cond.LastTransitionTime).NotTo(BeZero())
						Expect(cond.LastUpdateTime).NotTo(BeZero())
						Expect(cond.Reason).To(Or(Equal("NewFirewallSetAvailable"), Equal("FirewallSetUpdated")))
						Expect(cond.Message).To(Or(
							Equal(fmt.Sprintf("FirewallSet %q has successfully progressed.", newSet.Name)),
							Equal(fmt.Sprintf("Updated firewall set %q.", newSet.Name)),
						))
					})

					It("should populate the status", func() {
						var deploy = deployment()
						Eventually(func() int {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
							return deploy.Status.ReadyReplicas
						}, 15*time.Second, interval).Should(Equal(1), "reach 1 ready replicas")
						Eventually(func() int {
							Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deploy), deploy)).To(Succeed())
							return deploy.Status.ProgressingReplicas
						}, 5*time.Second, interval).Should(Equal(0), "reach 0 progressing replicas")

						Expect(deploy.Status.TargetReplicas).To(Equal(1))
						Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
						Expect(deploy.Status.ObservedRevision).To(Equal(1))
					})
				})
			})
		})

		Describe("the deletion flow", Ordered, func() {
			When("deleting the firewall deployment", func() {
				It("the deletion finishes", func() {
					swapMetalClient(&metalclient.MetalMockFns{
						Firewall: func(m *mock.Mock) {
							m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Machine: func(m *mock.Mock) {
							m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					Expect(k8sClient.Delete(ctx, deployment())).To(Succeed())

					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallDeploymentList{}, func(l *v2.FirewallDeploymentList) []*v2.FirewallDeployment {
						return l.GetItems()
					}, 10*time.Second)
				})
			})

			Context("all resources are cleaned up", func() {
				It("should delete the firewall set", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete the firewall", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete firewall monitor", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
						return l.GetItems()
					}, 10*time.Second)
				})
			})
		})
	})

	Context("migration path", Ordered, func() {
		var (
			fw = &v2.Firewall{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: namespaceName,
					Labels: map[string]string{
						"purpose": "shoot-firewall",
					},
					Annotations: map[string]string{
						v2.FirewallNoControllerConnectionAnnotation: "true",
					},
				},
				Spec: v2.FirewallSpec{
					Size:                    "n1-medium-x86",
					Project:                 "project-a",
					Partition:               "partition-a",
					Image:                   "firewall-ubuntu-2.0",
					Networks:                []string{"internet"},
					ControllerURL:           "http://controller.tar.gz",
					ControllerVersion:       "v2.0.0",
					NftablesExporterURL:     "http://exporter.tar.gz",
					NftablesExporterVersion: "v1.0.0",
				},
			}

			deployment = func() *v2.FirewallDeployment {
				return &v2.FirewallDeployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: namespaceName,
					},
					Spec: v2.FirewallDeploymentSpec{
						Replicas: 1,
						Template: v2.FirewallTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"purpose": "shoot-firewall",
								},
							},
							Spec: v2.FirewallSpec{
								Size:                    "n1-medium-x86",
								Project:                 "project-a",
								Partition:               "partition-a",
								Image:                   "firewall-ubuntu-2.0",
								Networks:                []string{"internet"},
								ControllerURL:           "http://controller.tar.gz",
								ControllerVersion:       "v2.0.0",
								NftablesExporterURL:     "http://exporter.tar.gz",
								NftablesExporterVersion: "v1.0.0",
							},
						},
					},
				}
			}
		)

		When("creating a firewall resource (for an existing firewall)", Ordered, func() {
			It("the creation works", func() {
				swapMetalClient(&metalclient.MetalMockFns{
					Firewall: func(m *mock.Mock) {
						m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
						m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
					},
					Network: func(m *mock.Mock) {
						m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
					},
					Machine: func(m *mock.Mock) {
						m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
					},
					Image: func(m *mock.Mock) {
						m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
					},
				})

				Expect(k8sClient.Create(ctx, fw)).To(Succeed())
			})

			Specify("no other resources pop up", func() {
				_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				}, 10*time.Second)
				_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallDeploymentList{}, func(l *v2.FirewallDeploymentList) []*v2.FirewallDeployment {
					return l.GetItems()
				}, 10*time.Second)

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fw), fw)).To(Succeed())
				Expect(fw.OwnerReferences).To(HaveLen(0))
			})
		})

		var (
			set *v2.FirewallSet
		)

		When("creating a firewall deployment", Ordered, func() {
			It("the creation works", func() {
				Expect(k8sClient.Create(ctx, deployment())).To(Succeed())
			})

			It("should create a firewall set", func() {
				set = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
					return l.GetItems()
				}, 15*time.Second)
			})

			It("should adopt the existing firewall", func() {
				fw = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
					return l.GetItems()
				}, 10*time.Second)

				Expect(fw.Name).To(Equal("test"))
				Expect(fw.OwnerReferences).To(HaveLen(1))

				Expect(fw.OwnerReferences[0].UID).To(Equal(set.UID))
			})

			It("should create a firewall monitor", func() {
				_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
					return l.GetItems()
				}, 5*time.Second)
			})
		})

		Context("the firewall resource", func() {
			It("should have the set as an owner", func() {
				Expect(fw.ObjectMeta.OwnerReferences).To(HaveLen(1))
				Expect(fw.ObjectMeta.OwnerReferences[0].Name).To(Equal(set.Name))
			})

			It("should have the created condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallCreated, v2.ConditionTrue, 15*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
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
				}, 5*time.Second, interval).Should(Not(BeNil()))

				Expect(status.MachineID).To(Equal(*firewall1.ID))
				Expect(status.CrashLoop).To(Equal(false))
				Expect(status.Liveliness).To(Equal("Alive"))
				Expect(status.LastEvent).NotTo(BeNil())
				Expect(status.LastEvent.Event).To(Equal("Phoned Home"))
				Expect(status.LastEvent.Message).To(Equal("phoning home"))
			})

			It("should have the ready condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallReady, v2.ConditionTrue, 15*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Equal("Ready"))
				Expect(cond.Message).To(Equal(fmt.Sprintf("Firewall %q is phoning home and alive.", *firewall1.Allocation.Name)))
			})

			It("should have the monitor condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallMonitorDeployed, v2.ConditionTrue, 5*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Equal("Deployed"))
				Expect(cond.Message).To(Equal("Successfully deployed firewall-monitor."))
			})

			It("should have the firewall-controller connected condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, fw.DeepCopy(), func(fd *v2.Firewall) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallControllerConnected, v2.ConditionTrue, 15*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Equal("NotChecking"))
				Expect(cond.Message).To(Equal("Not checking controller connection due to firewall annotation."))
			})
		})

		Context("the firewall set resource", func() {
			It("should have the deployment as an owner", func() {
				Expect(set.ObjectMeta.OwnerReferences).To(HaveLen(1))
				Expect(set.ObjectMeta.OwnerReferences[0].Name).To(Equal(deployment().Name))
			})

			It("should populate the status", func() {
				var set = set.DeepCopy()
				Eventually(func() int {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
					return set.Status.ReadyReplicas
				}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

				Expect(set.Status.TargetReplicas).To(Equal(1))
				Expect(set.Status.ProgressingReplicas).To(Equal(0))
				Expect(set.Status.UnhealthyReplicas).To(Equal(0))
				Expect(set.Status.ObservedRevision).To(Equal(0)) // this is the first revision
			})
		})

		Context("the firewall deployment resource", func() {
			It("should have the rbac condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallDeplomentRBACProvisioned, v2.ConditionTrue, 5*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Equal("Provisioned"))
				Expect(cond.Message).To(Equal("RBAC provisioned successfully."))
			})

			It("should have the available condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallDeplomentAvailable, v2.ConditionTrue, 5*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Equal("MinimumReplicasAvailable"))
				Expect(cond.Message).To(Equal("Deployment has minimum availability."))
			})

			It("should have the progress condition true", func() {
				cond := testcommon.WaitForCondition(k8sClient, ctx, deployment(), func(fd *v2.FirewallDeployment) v2.Conditions {
					return fd.Status.Conditions
				}, v2.FirewallDeplomentProgressing, v2.ConditionTrue, 30*time.Second)

				Expect(cond.LastTransitionTime).NotTo(BeZero())
				Expect(cond.LastUpdateTime).NotTo(BeZero())
				Expect(cond.Reason).To(Or(Equal("NewFirewallSetAvailable"), Equal("FirewallSetUpdated")))
				Expect(cond.Message).To(Or(
					Equal(fmt.Sprintf("FirewallSet %q has successfully progressed.", set.Name)),
					Equal(fmt.Sprintf("Updated firewall set %q.", set.Name)),
				))
			})

			It("should populate the status", func() {
				deploy := &v2.FirewallDeployment{}
				Eventually(func() int {
					Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment()), deploy)).To(Succeed())
					return deploy.Status.ReadyReplicas
				}, 15*time.Second, interval).Should(Equal(1), "reach ready replicas")

				Expect(deploy.Status.TargetReplicas).To(Equal(1))
				Expect(deploy.Status.ProgressingReplicas).To(Equal(0))
				Expect(deploy.Status.UnhealthyReplicas).To(Equal(0))
				Expect(deploy.Status.ObservedRevision).To(Equal(0)) // this is the first revision
			})
		})

		Describe("the deletion flow", Ordered, func() {
			When("deleting the firewall deployment", func() {
				It("the deletion finishes", func() {
					swapMetalClient(&metalclient.MetalMockFns{
						Firewall: func(m *mock.Mock) {
							m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall1}, nil).Maybe()
							m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall1}}, nil).Maybe()
						},
						Network: func(m *mock.Mock) {
							m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
						},
						Machine: func(m *mock.Mock) {
							m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall1.ID}}, nil).Maybe()
							m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
						},
						Image: func(m *mock.Mock) {
							m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
						},
					})

					Expect(k8sClient.Delete(ctx, deployment())).To(Succeed())

					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallDeploymentList{}, func(l *v2.FirewallDeploymentList) []*v2.FirewallDeployment {
						return l.GetItems()
					}, 10*time.Second)
				})
			})

			Context("all resources are cleaned up", func() {
				It("should delete the firewall set", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallSetList{}, func(l *v2.FirewallSetList) []*v2.FirewallSet {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete the firewall", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
						return l.GetItems()
					}, 10*time.Second)
				})

				It("should delete firewall monitor", func() {
					_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallMonitorList{}, func(l *v2.FirewallMonitorList) []*v2.FirewallMonitor {
						return l.GetItems()
					}, 10*time.Second)
				})
			})
		})

	})

	When("creating a firewall set that simulates unhealthiness", Ordered, func() {
		var firewallSet *v2.FirewallSet

		BeforeAll(func() {
			swapMetalClient(&metalclient.MetalMockFns{
				Firewall: func(m *mock.Mock) {
					m.On("AllocateFirewall", mock.Anything, nil).Return(&metalfirewall.AllocateFirewallOK{Payload: firewall3}, nil).Maybe()
					m.On("FindFirewall", mock.Anything, nil).Return(&metalfirewall.FindFirewallOK{Payload: firewall3}, nil).Maybe()
					m.On("FindFirewalls", mock.Anything, nil).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{firewall3}}, nil).Maybe()
				},
				Network: func(m *mock.Mock) {
					m.On("FindNetwork", mock.Anything, nil).Return(&network.FindNetworkOK{Payload: network1}, nil).Maybe()
				},
				Machine: func(m *mock.Mock) {
					m.On("UpdateMachine", mock.Anything, nil).Return(&machine.UpdateMachineOK{Payload: &models.V1MachineResponse{}}, nil).Maybe()
					m.On("FreeMachine", mock.Anything, nil).Return(&machine.FreeMachineOK{Payload: &models.V1MachineResponse{ID: firewall3.ID}}, nil).Maybe()
				},
				Image: func(m *mock.Mock) {
					m.On("FindLatestImage", mock.Anything, nil).Return(&image.FindLatestImageOK{Payload: image1}, nil).Maybe()
				},
			})

			Expect(k8sClient.Create(ctx, deployment())).To(Succeed())

			Eventually(func() error {
				firewallSetList := &v2.FirewallSetList{}
				err := k8sClient.List(ctx, firewallSetList, client.InNamespace(namespaceName))
				if err != nil {
					return err
				}
				if len(firewallSetList.Items) == 0 {
					return fmt.Errorf("no firewall sets found")
				}
				firewallSet = &firewallSetList.Items[0]
				return nil
			}, 15*time.Second, interval).Should(Succeed(), "FirewallSet should be created")
		})

		It("should simulate unhealthiness and trigger deletion", func() {
			firewallList := &v2.FirewallList{}
			Eventually(func() int {
				err := k8sClient.List(ctx, firewallList, client.InNamespace(firewallSet.Namespace))
				if err != nil {
					return 0
				}
				return len(firewallList.Items)
			}, 15*time.Second, interval).Should(BeNumerically(">", 0), "Should have at least one firewall")

			Eventually(func() error {
				for _, item := range firewallList.Items {
					var fw v2.Firewall
					err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&item), &fw)
					if err != nil {
						fmt.Printf("Failed to get firewall: %v\n", err)
						return err
					}

					if fw.Status.ControllerStatus == nil {
						fw.Status.ControllerStatus = &v2.ControllerConnection{}
					}
					//add a fake concile so the unhealty firewall gets deleted
					fw.Status.ControllerStatus.SeedUpdated.Time = time.Now().Add(-(firewallHealthTimeout + time.Minute))
					err = k8sClient.Status().Update(ctx, &fw)
					if err != nil {
						return fmt.Errorf("failed to update firewall status: %w", err)
					}
				}

				return nil
			}, 10*time.Second, interval).Should(Succeed(), "All Firewalls should be deleted")

			By("verifying that a new firewall has been created")
			Eventually(func() int {
				newFirewallList := &v2.FirewallList{}
				Expect(k8sClient.List(ctx, newFirewallList, client.InNamespace(firewallSet.Namespace))).To(Succeed())
				return len(newFirewallList.Items)
			}, 10*time.Second, interval).Should(Equal(1), "A new firewall should be created")
		})

	})

})
