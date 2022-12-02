package controllers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-logr/zapr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	"github.com/metal-stack/metal-lib/pkg/tag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	zcfg := zap.NewProductionConfig()
	zcfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	zcfg.EncoderConfig.TimeKey = "timestamp"
	zcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	l, err := zcfg.Build()
	Expect(err).NotTo(HaveOccurred())

	ctrl.SetLogger(zapr.NewLogger(l))

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

	err = (&deployment.Config{
		ControllerConfig: deployment.ControllerConfig{
			Seed:          k8sClient,
			Metal:         metalClient,
			Namespace:     namespaceName,
			ClusterID:     "cluster-a",
			ClusterTag:    fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			ClusterAPIURL: "http://shoot-api",
			K8sVersion:    semver.MustParse("v1.25.0"),
			Recorder:      mgr.GetEventRecorderFor("firewall-deployment-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("deployment"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  k8sClient,
			Metal:                 metalClient,
			Namespace:             namespaceName,
			ClusterID:             "cluster-a",
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			FirewallHealthTimeout: 20 * time.Minute,
			Recorder:              mgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:           k8sClient,
			Shoot:          k8sClient,
			Metal:          metalClient,
			Namespace:      namespaceName,
			ShootNamespace: v2.FirewallShootNamespace,
			ClusterID:      "cluster-a",
			ClusterTag:     fmt.Sprintf("%s=%s", tag.ClusterID, "cluster-a"),
			Recorder:       mgr.GetEventRecorderFor("firewall-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}).SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	err = (&monitor.Config{
		ControllerConfig: monitor.ControllerConfig{
			Seed:          k8sClient,
			Shoot:         k8sClient,
			Namespace:     v2.FirewallShootNamespace,
			SeedNamespace: namespaceName,
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall-monitor"),
	}).SetupWithManager(mgr)
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
