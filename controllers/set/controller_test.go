package set_test

import (
	"fmt"
	"strconv"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	testcommon "github.com/metal-stack/firewall-controller-manager/integration/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
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
)

var _ = Context("firewall set controller", Ordered, func() {
	var (
		set = &v2.FirewallSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespaceName,
			},
			Spec: v2.FirewallSetSpec{
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
						ControllerVersion:       "v0.0.1",
						NftablesExporterURL:     "http://exporter.tar.gz",
						NftablesExporterVersion: "v1.0.0",
					},
				},
			},
		}
	)

	BeforeAll(func() {
		Expect(client.IgnoreAlreadyExists(k8sClient.Create(ctx, sshSecret.DeepCopy()))).To(Succeed())
	})

	When("creating a three replica firewall set", Ordered, func() {
		It("the creation works", func() {
			set.Spec.Replicas = 3
			Expect(k8sClient.Create(ctx, set)).To(Succeed())
		})

		It("should create three firewalls", func() {
			_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 3, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
				return l.GetItems()
			}, 3*time.Second)
		})
	})

	When("when scaling up to four replicas", Ordered, func() {
		time.Sleep(1 * time.Second) // make time gap between latest and older firewalls easier to spot

		It("the update works", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
			set.Spec.Replicas = 4
			Expect(k8sClient.Update(ctx, set)).To(Succeed())
		})

		It("should create another firewall", func() {
			_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 4, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
				return l.GetItems()
			}, 3*time.Second)
		})
	})

	When("scaling down the set to one", Ordered, func() {
		It("it keeps the firewall with the newest timestamp", func() {
			fws := &v2.FirewallList{}
			Expect(k8sClient.List(ctx, fws, client.InNamespace(namespaceName))).To(Succeed())
			Expect(fws.Items).To(HaveLen(4))

			var newest *v2.Firewall
			for _, fw := range fws.Items {
				_, _ = fmt.Fprintf(GinkgoWriter, "Having %s with creation timestamp: %s\n", fw.Name, fw.CreationTimestamp.String())

				if newest == nil {
					newest = &fw
				}
				if !fw.CreationTimestamp.Time.Before(newest.CreationTimestamp.Time) {
					newest = &fw
				}
			}

			Expect(newest).NotTo(BeNil())
			_, _ = fmt.Fprintf(GinkgoWriter, "The latest firewall is: %s\n", newest.Name)

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
			set.Spec.Replicas = 1
			Expect(k8sClient.Update(ctx, set)).To(Succeed())

			firewall := testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 1, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
				return l.GetItems()
			}, 5*time.Second)

			Expect(firewall.Name).To(Equal(newest.Name), "older firewalls were kept")
		})
	})

	Describe("scale down to zero is also working", Ordered, func() {
		It("the update works", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
			set.Spec.Replicas = 0
			Expect(k8sClient.Update(ctx, set)).To(Succeed())
		})

		It("should delete the last firewall", func() {
			_ = testcommon.WaitForResourceAmount(k8sClient, ctx, namespaceName, 0, &v2.FirewallList{}, func(l *v2.FirewallList) []*v2.Firewall {
				return l.GetItems()
			}, 3*time.Second)
		})
	})

	Describe("scaling beneath zero does not make any sense though", Ordered, func() {
		It("the update does not work", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
			set.Spec.Replicas = -1
			Expect(k8sClient.Update(ctx, set)).NotTo(Succeed())
		})
	})

	Describe("reconcile annotation", Ordered, func() {
		It("the annotation can be added", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
			if set.Annotations == nil {
				set.Annotations = map[string]string{}
			}
			set.Annotations[v2.ReconcileAnnotation] = strconv.FormatBool(true)
			Expect(k8sClient.Update(ctx, set)).To(Succeed())
		})

		It("the annotation was cleaned up again", func() {
			Eventually(func() bool {
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), set)).To(Succeed())
				_, present := set.Annotations[v2.ReconcileAnnotation]
				return present
			}, "50ms").Should(BeFalse())
		})
	})
})
