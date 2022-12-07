package set_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/zapr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	metalfirewall "github.com/metal-stack/metal-go/api/client/firewall"
	"github.com/metal-stack/metal-go/api/models"
	metalclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/tag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	//+kubebuilder:scaffold:imports
)

const (
	namespaceName = "default"
)

var (
	ctx        context.Context
	cancel     context.CancelFunc
	k8sClient  client.Client
	testEnv    *envtest.Environment
	configPath = filepath.Join("..", "..", "config")
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	l, err := controllers.NewZapLogger("debug")
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(zapr.NewLogger(l.Desugar()))

	ctx, cancel = context.WithCancel(context.Background())

	By("bootstrapping test environment")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join(configPath, "crds")},
		ErrorIfCRDPathMissing: true,
		// AttachControlPlaneOutput: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join(configPath, "webhooks")},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = v2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		CertDir:            testEnv.WebhookInstallOptions.LocalServingCertDir,
		Host:               testEnv.WebhookInstallOptions.LocalServingHost,
		Port:               testEnv.WebhookInstallOptions.LocalServingPort,
	})
	Expect(err).ToNot(HaveOccurred())

	_, metalClient := metalclient.NewMetalMockClient(&metalclient.MetalMockFns{
		Firewall: func(m *mock.Mock) {
			// muting the orphan controller
			m.On("FindFirewalls", mock.Anything, mock.Anything).Return(&metalfirewall.FindFirewallsOK{Payload: []*models.V1FirewallResponse{}}, nil)
		},
	})

	setConfig := &set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  k8sClient,
			Metal:                 metalClient,
			Namespace:             namespaceName,
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			FirewallHealthTimeout: 20 * time.Minute,
			CreateTimeout:         10 * time.Minute,
			Recorder:              mgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}
	err = setConfig.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())
	err = setConfig.SetupWebhookWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	firewallConfig := &firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:           k8sClient,
			Shoot:          k8sClient,
			Metal:          metalClient,
			Namespace:      namespaceName,
			ShootNamespace: v2.FirewallShootNamespace,
			APIServerURL:   "http://shoot-api",
			ClusterTag:     fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			Recorder:       mgr.GetEventRecorderFor("firewall-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}
	err = firewallConfig.SetupWebhookWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	//+kubebuilder:scaffold:scheme
	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
