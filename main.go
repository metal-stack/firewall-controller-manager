package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
)

const (
	metalAPIUrlEnvVar = "METAL_API_URL"
	// nolint
	metalAuthTokenEnvVar = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar  = "METAL_AUTH_HMAC"
)

func main() {
	var (
		scheme = runtime.NewScheme()

		logLevel                string
		metricsAddr             string
		enableLeaderElection    bool
		shootKubeconfig         string
		namespace               string
		gracefulShutdownTimeout time.Duration
		reconcileInterval       time.Duration
		firewallHealthTimeout   time.Duration
		createTimeout           time.Duration
		safetyBackoff           time.Duration
		progressDeadline        time.Duration
		clusterID               string
		clusterApiURL           string
		certDir                 string
	)

	flag.StringVar(&logLevel, "log-level", "info", "the log level of the controller")
	flag.StringVar(&metricsAddr, "metrics-addr", ":2112", "the address the metric endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager")
	flag.StringVar(&namespace, "namespace", "default", "the namespace this controller is running")
	flag.StringVar(&shootKubeconfig, "shoot-kubeconfig", "", "the path to the kubeconfig to talk to the shoot")
	flag.DurationVar(&reconcileInterval, "reconcile-interval", 1*time.Minute, "duration after which a resource is getting reconciled at minimum")
	flag.DurationVar(&firewallHealthTimeout, "firewall-health-timeout", 20*time.Minute, "duration after a created firewall not getting ready is considered dead")
	flag.DurationVar(&createTimeout, "create-timeout", 10*time.Minute, "duration after which a firewall in the creation phase will be recreated")
	flag.DurationVar(&safetyBackoff, "safety-backoff", 10*time.Second, "duration after which a resource is getting reconciled at minimum")
	flag.DurationVar(&progressDeadline, "progress-deadline", 15*time.Minute, "time after which a deployment is considered unhealthy instead of progressing (informational)")
	flag.DurationVar(&gracefulShutdownTimeout, "graceful-shutdown-timeout", -1, "grace period after which the controller shuts down")
	flag.StringVar(&clusterID, "cluster-id", "", "id of the cluster this controller is responsible for")
	flag.StringVar(&clusterApiURL, "cluster-api-url", "", "url of the cluster to put into the kubeconfig")
	flag.StringVar(&certDir, "cert-dir", "", "the directory that contains the server key and certificate for the webhook server")

	flag.Parse()

	l, err := controllers.NewZapLogger(logLevel)
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to parse log level")
		os.Exit(1)
	}
	ctrl.SetLogger(zapr.NewLogger(l.Desugar()))

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v2.AddToScheme(scheme))

	var (
		restConfig      = ctrl.GetConfigOrDie()
		discoveryClient = discovery.NewDiscoveryClientForConfigOrDie(restConfig)

		shootClient, _  = client.New(restConfig, client.Options{Scheme: scheme}) // defaults to seed, e.g. for devel purposes
		shootRestConfig = restConfig                                             // defaults to seed, e.g. for devel purposes
	)

	if shootKubeconfig == "" {
		l.Infow("no shoot kubeconfig configured, running in single-cluster mode")
	} else {
		l.Infow("shoot kubeconfig configured, running in split-cluster mode (seed/shoot)")

		shootRestConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: shootKubeconfig},
			&clientcmd.ConfigOverrides{},
		).ClientConfig()
		if err != nil {
			l.Fatalw("unable to create shoot restconfig", "error", err)
		}

		shootClient, err = client.New(shootRestConfig, client.Options{Scheme: scheme})
		if err != nil {
			l.Fatalw("unable to create shoot client", "error", err)
		}
	}

	mclient, err := getMetalClient()
	if err != nil {
		l.Fatalw("unable to create metal client", "error", err)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		l.Fatalw("unable to discover server version", "error", err)
	}

	l.Infow("seed kubernetes version", "version", version.String())

	k8sVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		l.Fatalw("unable to parse kubernetes version version", "error", err)
	}

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
		l.Fatalw("unable to start firewall-controller-manager", "error", err)
	}

	deploymentConfig := &deployment.Config{
		ControllerConfig: deployment.ControllerConfig{
			Seed:             mgr.GetClient(),
			Metal:            mclient,
			Namespace:        namespace,
			ClusterAPIURL:    clusterApiURL,
			K8sVersion:       k8sVersion,
			Recorder:         mgr.GetEventRecorderFor("firewall-deployment-controller"),
			SafetyBackoff:    safetyBackoff,
			ProgressDeadline: progressDeadline,
		},
		Log: ctrl.Log.WithName("controllers").WithName("deployment"),
	}
	if err = deploymentConfig.SetupWithManager(mgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "deployment")
	}
	if err = deploymentConfig.SetupWebhookWithManager(mgr); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "deployment")
	}

	setConfig := &set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  mgr.GetClient(),
			Metal:                 mclient,
			Namespace:             namespace,
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			FirewallHealthTimeout: firewallHealthTimeout,
			CreateTimeout:         createTimeout,
			Recorder:              mgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}
	if err = setConfig.SetupWithManager(mgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "set")
	}
	if err = setConfig.SetupWebhookWithManager(mgr); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "set")
	}

	firewallConfig := &firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:           mgr.GetClient(),
			Shoot:          shootClient,
			Metal:          mclient,
			Namespace:      namespace,
			ShootNamespace: v2.FirewallShootNamespace,
			ClusterTag:     fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			Recorder:       mgr.GetEventRecorderFor("firewall-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}
	if err = firewallConfig.SetupWithManager(mgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "firewall")
	}
	if err = firewallConfig.SetupWebhookWithManager(mgr); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "firewall")
	}

	shootMgr, err := ctrl.NewManager(shootRestConfig, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      "0",
		LeaderElection:          false,
		Namespace:               v2.FirewallShootNamespace,
		GracefulShutdownTimeout: pointer.Pointer(time.Duration(0)),
	})
	if err != nil {
		l.Fatalw("unable to start firewall-controller-manager-monitor", "error", err)
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
		l.Fatalw("unable to setup controller", "error", err, "controller", "monitor")
	}

	stop := ctrl.SetupSignalHandler()

	go func() {
		l.Infow("starting shoot controller", "version", v.V)
		if err := shootMgr.Start(stop); err != nil {
			l.Fatalw("problem running shoot controller", "error", err)
		}
	}()

	l.Infow("starting seed controller", "version", v.V)
	if err := mgr.Start(stop); err != nil {
		l.Fatalw("problem running seed controller", "error", err)
	}
}

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
