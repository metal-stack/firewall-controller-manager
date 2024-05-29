package controllers_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	controllerconfig "github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	"github.com/metal-stack/firewall-controller-manager/controllers/update"
	metalclient "github.com/metal-stack/metal-go/test/client"
	"github.com/metal-stack/metal-lib/pkg/tag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

const (
	namespaceName = "test"
)

var (
	ctx        context.Context
	cancel     context.CancelFunc
	k8sClient  client.Client
	testEnv    *envtest.Environment
	configPath = filepath.Join("..", "config")
	apiHost    string
	apiCA      string
	apiCert    string
	apiKey     string
	testingT   *testing.T
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

	apiHost = cfg.Host
	apiCert = base64.StdEncoding.EncodeToString(cfg.CertData)
	apiKey = base64.StdEncoding.EncodeToString(cfg.KeyData)
	apiCA = base64.StdEncoding.EncodeToString(cfg.CAData)

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
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		LeaderElection: false,
	})
	Expect(err).ToNot(HaveOccurred())

	metalClient, _ = metalclient.NewMetalMockClient(testingT, nil)

	cc, err := controllerconfig.New(&controllerconfig.NewControllerConfig{
		SeedClient:        k8sClient,
		SeedConfig:        cfg,
		SeedNamespace:     namespaceName,
		SeedAPIServerURL:  apiHost,
		ShootClient:       k8sClient,
		ShootConfig:       cfg,
		ShootNamespace:    namespaceName,
		ShootAPIServerURL: apiHost,
		ShootAccess: &v2.ShootAccess{
			GenericKubeconfigSecretName: "kubeconfig-secret-name",
			TokenSecretName:             "token",
			Namespace:                   namespaceName,
			APIServerURL:                apiHost,
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

	err = deployment.SetupWithManager(
		ctrl.Log.WithName("controllers").WithName("deployment"),
		mgr.GetEventRecorderFor("firewall-deployment-controller"),
		mgr,
		cc,
	)
	Expect(err).ToNot(HaveOccurred())

	err = set.SetupWithManager(
		ctrl.Log.WithName("controllers").WithName("set"),
		mgr.GetEventRecorderFor("firewall-set-controller"),
		mgr,
		cc,
	)
	Expect(err).ToNot(HaveOccurred())

	err = firewall.SetupWithManager(
		ctrl.Log.WithName("controllers").WithName("firewall"),
		mgr.GetEventRecorderFor("firewall-controller"),
		mgr,
		cc,
	)
	Expect(err).ToNot(HaveOccurred())

	err = update.SetupWithManager(
		ctrl.Log.WithName("controllers").WithName("update"),
		mgr.GetEventRecorderFor("update-controller"),
		mgr,
		cc,
	)
	Expect(err).ToNot(HaveOccurred())

	err = deployment.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), mgr, cc)
	Expect(err).ToNot(HaveOccurred())
	err = set.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), mgr, cc)
	Expect(err).ToNot(HaveOccurred())
	err = firewall.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), mgr, cc)
	Expect(err).ToNot(HaveOccurred())

	err = monitor.SetupWithManager(ctrl.Log.WithName("controllers").WithName("firewall-monitor"), mgr, cc)
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
