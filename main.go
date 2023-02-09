package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/discovery"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/zapr"
	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/metal-stack/v"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/defaults"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
)

const (
	metalAuthHMACEnvVar = "METAL_AUTH_HMAC"
)

func main() {
	var (
		scheme   = runtime.NewScheme()
		metalURL string

		logLevel                string
		metricsAddr             string
		enableLeaderElection    bool
		shootKubeconfigSecret   string
		shootTokenSecret        string
		sshKeySecret            string
		namespace               string
		gracefulShutdownTimeout time.Duration
		reconcileInterval       time.Duration
		firewallHealthTimeout   time.Duration
		createTimeout           time.Duration
		safetyBackoff           time.Duration
		progressDeadline        time.Duration
		clusterID               string
		shootApiURL             string
		seedApiURL              string
		certDir                 string
	)

	flag.StringVar(&logLevel, "log-level", "info", "the log level of the controller")
	flag.StringVar(&metricsAddr, "metrics-addr", ":2112", "the address the metric endpoint binds to")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager")
	flag.StringVar(&namespace, "namespace", "default", "the namespace this controller is running")
	flag.DurationVar(&reconcileInterval, "reconcile-interval", 10*time.Minute, "duration after which a resource is getting reconciled at minimum")
	flag.DurationVar(&firewallHealthTimeout, "firewall-health-timeout", 20*time.Minute, "duration after a created firewall not getting ready is considered dead")
	flag.DurationVar(&createTimeout, "create-timeout", 10*time.Minute, "duration after which a firewall in the creation phase will be recreated")
	flag.DurationVar(&safetyBackoff, "safety-backoff", 10*time.Second, "duration after which a resource is getting reconciled at minimum")
	flag.DurationVar(&progressDeadline, "progress-deadline", 15*time.Minute, "time after which a deployment is considered unhealthy instead of progressing (informational)")
	flag.DurationVar(&gracefulShutdownTimeout, "graceful-shutdown-timeout", -1, "grace period after which the controller shuts down")
	flag.StringVar(&metalURL, "metal-api-url", "", "the url of the metal-stack api")
	flag.StringVar(&clusterID, "cluster-id", "", "id of the cluster this controller is responsible for")
	flag.StringVar(&shootApiURL, "shoot-api-url", "", "url of the shoot api server")
	flag.StringVar(&seedApiURL, "seed-api-url", "", "url of the seed api server")
	flag.StringVar(&certDir, "cert-dir", "", "the directory that contains the server key and certificate for the webhook server")
	flag.StringVar(&shootKubeconfigSecret, "shoot-kubeconfig-secret-name", "", "the secret name of the generic kubeconfig for shoot access")
	flag.StringVar(&shootTokenSecret, "shoot-token-secret-name", "", "the secret name of the token for shoot access")
	flag.StringVar(&sshKeySecret, "ssh-key-secret-name", "", "the secret name of the ssh key for machine access")

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
		seedConfig      = ctrl.GetConfigOrDie()
		shootConfig     = seedConfig // defaults to seed, e.g. for devel purposes
		discoveryClient = discovery.NewDiscoveryClientForConfigOrDie(seedConfig)
		stop            = ctrl.SetupSignalHandler()
		shootAccess     = &v2.ShootAccess{
			GenericKubeconfigSecretName: shootKubeconfigSecret,
			TokenSecretName:             shootTokenSecret,
			Namespace:                   namespace,
			APIServerURL:                shootApiURL,
			SSHKeySecretName:            sshKeySecret,
		}
	)

	mclient, err := getMetalClient(metalURL)
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

	seedMgr, err := ctrl.NewManager(seedConfig, ctrl.Options{
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

	if shootKubeconfigSecret == "" && shootTokenSecret == "" {
		l.Infow("no shoot kubeconfig configured, running in single-cluster mode (dev mode)")

		shootKubeconfigSecret = "dev-mode"
		shootTokenSecret = "dev-mode"
		sshKeySecret = "dev-mode"
	} else {
		l.Infow("shoot kubeconfig configured, running in split-cluster mode (seed/shoot)")

		// cannot use seedMgr.GetClient() because it gets initialized at a later point in time
		// we have to create an own client
		client, err := controllerclient.New(seedConfig, controllerclient.Options{
			Scheme: scheme,
		})
		if err != nil {
			l.Fatalw("unable to create seed client", "error", err)
		}

		var expiresAt *time.Time

		expiresAt, _, shootConfig, err = helper.NewShootConfig(context.Background(), client, shootAccess)
		if err != nil {
			l.Fatalw("unable to create shoot client", "error", err)
		}

		// as we are creating the client without projected token mount and tokenfile,
		// we need to regularly check for token expiration and restart the controller if necessary
		// in order to recreate the shoot client.
		helper.ShutdownOnTokenExpiration(ctrl.Log.WithName("token-expiration"), expiresAt, stop)
	}

	defaulterConfig := &defaults.DefaulterConfig{
		Log:         ctrl.Log.WithName("defaulting-webhook"),
		Seed:        seedMgr.GetClient(),
		Namespace:   namespace,
		K8sVersion:  k8sVersion,
		ShootAccess: shootAccess,
	}

	deploymentConfig := &deployment.Config{
		ControllerConfig: deployment.ControllerConfig{
			Seed:             seedMgr.GetClient(),
			Metal:            mclient,
			Namespace:        namespace,
			K8sVersion:       k8sVersion,
			Recorder:         seedMgr.GetEventRecorderFor("firewall-deployment-controller"),
			SafetyBackoff:    safetyBackoff,
			ProgressDeadline: progressDeadline,
			ShootAccess:      shootAccess,
		},
		Log: ctrl.Log.WithName("controllers").WithName("deployment"),
	}
	if err := deploymentConfig.SetupWithManager(seedMgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "deployment")
	}
	if err := deploymentConfig.SetupWebhookWithManager(seedMgr, defaulterConfig); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "deployment")
	}

	setConfig := &set.Config{
		ControllerConfig: set.ControllerConfig{
			Seed:                  seedMgr.GetClient(),
			Metal:                 mclient,
			Namespace:             namespace,
			ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			FirewallHealthTimeout: firewallHealthTimeout,
			CreateTimeout:         createTimeout,
			Recorder:              seedMgr.GetEventRecorderFor("firewall-set-controller"),
		},
		Log: ctrl.Log.WithName("controllers").WithName("set"),
	}
	if err := setConfig.SetupWithManager(seedMgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "set")
	}
	if err := setConfig.SetupWebhookWithManager(seedMgr, defaulterConfig); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "set")
	}

	shootMgr, err := ctrl.NewManager(shootConfig, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      "0",
		LeaderElection:          false,
		Namespace:               v2.FirewallShootNamespace,
		GracefulShutdownTimeout: pointer.Pointer(time.Duration(0)),
	})
	if err != nil {
		l.Fatalw("unable to start firewall-controller-manager-monitor", "error", err)
	}

	firewallConfig := &firewall.Config{
		ControllerConfig: firewall.ControllerConfig{
			Seed:                      seedMgr.GetClient(),
			Shoot:                     shootMgr.GetClient(),
			Metal:                     mclient,
			Namespace:                 namespace,
			ShootNamespace:            v2.FirewallShootNamespace,
			ClusterTag:                fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
			Recorder:                  seedMgr.GetEventRecorderFor("firewall-controller"),
			APIServerURL:              shootApiURL,
			ShootKubeconfigSecretName: shootKubeconfigSecret,
			ShootTokenSecretName:      shootTokenSecret,
			SSHKeySecretName:          sshKeySecret,
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall"),
	}
	if err := firewallConfig.SetupWithManager(seedMgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "firewall")
	}
	if err := firewallConfig.SetupWebhookWithManager(seedMgr, defaulterConfig); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "firewall")
	}

	monitorConfig := &monitor.Config{
		ControllerConfig: monitor.ControllerConfig{
			Seed:          seedMgr.GetClient(),
			Shoot:         shootMgr.GetClient(),
			Namespace:     v2.FirewallShootNamespace,
			SeedNamespace: namespace,
			K8sVersion:    k8sVersion,
			APIServerURL:  seedApiURL,
		},
		Log: ctrl.Log.WithName("controllers").WithName("firewall-monitor"),
	}
	if err := monitorConfig.SetupWithManager(shootMgr); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "monitor")
	}

	go func() {
		l.Infow("starting shoot controller", "version", v.V)
		if err := shootMgr.Start(stop); err != nil {
			l.Fatalw("problem running shoot controller", "error", err)
		}
	}()

	l.Infow("starting seed controller", "version", v.V)
	if err := seedMgr.Start(stop); err != nil {
		l.Fatalw("problem running seed controller", "error", err)
	}
}

func getMetalClient(url string) (metalgo.Client, error) {
	hmac := os.Getenv(metalAuthHMACEnvVar)

	if url == "" {
		return nil, fmt.Errorf("metal api url is required")
	}
	if hmac == "" {
		return nil, fmt.Errorf("environment variable %q is required", metalAuthHMACEnvVar)
	}

	client, err := metalgo.NewDriver(url, "", hmac)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize metal ccm:%w", err)
	}

	return client, nil
}
