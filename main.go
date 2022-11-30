package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/zapr"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/metal-stack/metal-lib/rest"
	"github.com/metal-stack/v"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v2.AddToScheme(scheme))
}

func main() {
	var (
		logLevel                string
		metricsAddr             string
		enableLeaderElection    bool
		shootKubeconfig         string
		namespace               string
		gracefulShutdownTimeout time.Duration
		reconcileInterval       time.Duration
		firewallHealthTimeout   time.Duration
		clusterID               string
		clusterApiURL           string
		certDir                 string
	)
	flag.StringVar(&logLevel, "log-level", "", "the log level of the controller")
	flag.StringVar(&metricsAddr, "metrics-addr", ":2112", "the address the metric endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager")
	flag.StringVar(&namespace, "namespace", "default", "the namespace this controller is running")
	flag.StringVar(&shootKubeconfig, "shoot-kubeconfig", "", "the path to the kubeconfig to talk to the shoot")
	flag.DurationVar(&reconcileInterval, "reconcile-interval", 1*time.Minute, "duration after which a resource is getting reconciled at minimum")
	flag.DurationVar(&firewallHealthTimeout, "firewall-health-timeout", 20*time.Minute, "duration after a created firewall not getting ready is considered dead")
	flag.DurationVar(&gracefulShutdownTimeout, "graceful-shutdown-timeout", -1, "grace period after which the controller shuts down")
	flag.StringVar(&clusterID, "cluster-id", "", "id of the cluster this controller is responsible for")
	flag.StringVar(&clusterApiURL, "cluster-api-url", "", "url of the cluster to put into the kubeconfig")
	flag.StringVar(&certDir, "cert-dir", "", "the directory that contains the server key and certificate for the webhook server")

	flag.Parse()

	level := zap.InfoLevel
	if len(logLevel) > 0 {
		err := level.UnmarshalText([]byte(logLevel))
		if err != nil {
			setupLog.Error(err, "can't initialize zap logger")
			os.Exit(1)
		}
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	l, err := cfg.Build()
	if err != nil {
		setupLog.Error(err, "can't initialize zap logger")
		os.Exit(1)
	}

	ctrl.SetLogger(zapr.NewLogger(l))
	if clusterID == "" {
		setupLog.Error(fmt.Errorf("cluster-id is not set"), "")
		os.Exit(1)
	}
	if clusterApiURL == "" {
		setupLog.Error(fmt.Errorf("cluster-api-url is not set"), "")
		os.Exit(1)
	}

	restConfig := ctrl.GetConfigOrDie()

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		Port:                    9443,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "firewall-controller-manager-leader-election",
		Namespace:               namespace,
		GracefulShutdownTimeout: &gracefulShutdownTimeout,
		SyncPeriod:              &reconcileInterval,
		CertDir:                 certDir,
	})
	if err != nil {
		setupLog.Error(err, "unable to start firewall-controller-manager")
		os.Exit(1)
	}

	var (
		discoveryClient = discovery.NewDiscoveryClientForConfigOrDie(mgr.GetConfig())

		shootClient, _  = client.New(restConfig, client.Options{Scheme: scheme}) // defaults to seed, e.g. for devel purposes
		shootRestConfig = restConfig                                             // defaults to seed, e.g. for devel purposes
	)
	if len(shootKubeconfig) > 0 {
		shootRestConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: shootKubeconfig},
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			setupLog.Error(err, "unable to create shoot restconfig")
			os.Exit(1)
		}
		shootClient, err = client.New(shootRestConfig, client.Options{Scheme: scheme})
		if err != nil {
			setupLog.Error(err, "unable to create shoot client")
			os.Exit(1)
		}
	}

	shootMgr, err := ctrl.NewManager(shootRestConfig, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      "0",
		LeaderElection:          false,
		Namespace:               v2.FirewallShootNamespace,
		GracefulShutdownTimeout: pointer.Pointer(time.Duration(0)),
	})
	if err != nil {
		setupLog.Error(err, "unable to start firewall-controller-manager-monitor")
		os.Exit(1)
	}

	mclient, err := getMetalClient()
	if err != nil {
		setupLog.Error(err, "unable to create metal client")
		os.Exit(1)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		setupLog.Error(err, "unable to discover server version")
		os.Exit(1)
	}

	setupLog.Info("seed kubernetes version", "version", version.String())
	k8sVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		setupLog.Error(err, "unable to parse kubernetes version version")
		os.Exit(1)
	}

	// TODO: deploy crds automatically, firewallmonitor to shoot, rest to seed

	if err = (&deployment.Config{
		ControllerConfig: deployment.ControllerConfig{
			Seed:          mgr.GetClient(),
			Metal:         mclient,
			Namespace:     namespace,
			ClusterID:     clusterID,
			ClusterTag:    fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			ClusterAPIURL: clusterApiURL,
			K8sVersion:    k8sVersion,
			Recorder:      mgr.GetEventRecorderFor("firewall-deployment-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("deployment"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "deployment")
		os.Exit(1)
	}

	if err = (&set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  mgr.GetClient(),
			Metal:                 mclient,
			Namespace:             namespace,
			ClusterID:             clusterID,
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			ClusterAPIURL:         clusterApiURL,
			FirewallHealthTimeout: firewallHealthTimeout,
			Recorder:              mgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "set")
		os.Exit(1)
	}

	if err = (&firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:           mgr.GetClient(),
			Shoot:          shootClient,
			Metal:          mclient,
			Namespace:      namespace,
			ShootNamespace: v2.FirewallShootNamespace,
			ClusterID:      clusterID,
			ClusterTag:     fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			Recorder:       mgr.GetEventRecorderFor("firewall-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "firewall")
		os.Exit(1)
	}

	if err = (&monitor.Config{
		ControllerConfig: monitor.ControllerConfig{
			Seed:          mgr.GetClient(),
			Shoot:         shootClient,
			Namespace:     v2.FirewallShootNamespace,
			SeedNamespace: namespace,
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall-monitor"),
	}).SetupWithManager(shootMgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "monitor")
		os.Exit(1)
	}

	stop := ctrl.SetupSignalHandler()

	go func() {
		setupLog.Info("starting firewall-controller-manager-monitor", "version", v.V)
		if err := shootMgr.Start(stop); err != nil {
			setupLog.Error(err, "problem running firewall-controller-manager-monitor")
			os.Exit(1)
		}
	}()

	setupLog.Info("starting firewall-controller-manager", "version", v.V)
	if err := mgr.Start(stop); err != nil {
		setupLog.Error(err, "problem running firewall-controller-manager")
		os.Exit(1)
	}
}

const (
	metalAPIUrlEnvVar = "METAL_API_URL"
	// nolint
	metalAuthTokenEnvVar = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar  = "METAL_AUTH_HMAC"
)

func getMetalClient() (metalgo.Client, error) {
	url := os.Getenv(metalAPIUrlEnvVar)
	token := os.Getenv(metalAuthTokenEnvVar)
	hmac := os.Getenv(metalAuthHMACEnvVar)

	if url == "" {
		return nil, fmt.Errorf("environment variable %q is required", metalAPIUrlEnvVar)
	}

	if (token == "") == (hmac == "") {
		return nil, fmt.Errorf("environment variable %q or %q is required", metalAuthTokenEnvVar, metalAuthHMACEnvVar)
	}

	var err error
	client, err := metalgo.NewDriver(url, token, hmac)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize metal ccm:%w", err)
	}

	resp, err := client.Health().Health(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("metal-api health endpoint not reachable:%w", err)
	}
	if resp.Payload != nil && resp.Payload.Status != nil && *resp.Payload.Status != string(rest.HealthStatusHealthy) {
		return nil, fmt.Errorf("metal-api not healthy, restarting")
	}
	return client, nil
}
