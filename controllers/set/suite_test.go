package set_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	controllerconfig "github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	metalclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/tag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

const (
	namespaceName = "default"
)

var (
	testingT   *testing.T
	ctx        context.Context
	cancel     context.CancelFunc
	k8sClient  client.Client
	testEnv    *envtest.Environment
	configPath = filepath.Join("..", "..", "config")
)

func TestAPIs(t *testing.T) {
	testingT = t
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	l, err := controllers.NewLogger("debug")
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(logr.FromSlogHandler(l))

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
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
		}),
		Metrics: server.Options{
			BindAddress: "0",
		},
		LeaderElection: false,
	})
	Expect(err).ToNot(HaveOccurred())

	_, metalClient := metalclient.NewMetalMockClient(testingT, &metalclient.MetalMockFns{})

	cc, err := controllerconfig.New(&controllerconfig.NewControllerConfig{
		SeedClient:        k8sClient,
		SeedConfig:        cfg,
		SeedNamespace:     namespaceName,
		SeedAPIServerURL:  cfg.Host,
		ShootClient:       k8sClient,
		ShootConfig:       cfg,
		ShootNamespace:    namespaceName,
		ShootAPIServerURL: cfg.Host,
		ShootAccess: &v2.ShootAccess{
			GenericKubeconfigSecretName: "generic",
			TokenSecretName:             "token-secret",
			Namespace:                   namespaceName,
			APIServerURL:                cfg.Host,
		},
		SSHKeySecretName:      sshSecret.Name,
		SSHKeySecretNamespace: sshSecret.Namespace,
		Metal:                 metalClient,
		ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
		SafetyBackoff:         10 * time.Second,
		ProgressDeadline:      10 * time.Minute,
		FirewallHealthTimeout: 20 * time.Minute,
		CreateTimeout:         10 * time.Minute,
	})
	Expect(err).ToNot(HaveOccurred())

	err = set.SetupWithManager(
		ctrl.Log.WithName("controllers").WithName("set"),
		mgr.GetEventRecorder("firewall-set-controller"),
		mgr,
		cc,
	)
	Expect(err).ToNot(HaveOccurred())
	err = set.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), mgr, cc)
	Expect(err).ToNot(HaveOccurred())

	err = firewall.SetupWebhookWithManager(ctrl.Log.WithName("controllers").WithName("firewall"), mgr, cc)
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
