package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/zapr"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/metal-stack/v"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
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
		scheme   = helper.MustNewFirewallScheme()
		metalURL string

		logLevel                string
		metricsAddr             string
		enableLeaderElection    bool
		shootKubeconfigSecret   string
		shootTokenSecret        string
		shootTokenPath          string
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
	flag.StringVar(&shootTokenPath, "shoot-token-path", "/", "the path where to store the token file for shoot access")

	flag.Parse()

	l, err := controllers.NewZapLogger(logLevel)
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to parse log level")
		os.Exit(1)
	}
	ctrl.SetLogger(zapr.NewLogger(l.Desugar()))

	var (
		stop        = ctrl.SetupSignalHandler()
		shootAccess = &v2.ShootAccess{
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

	seedMgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
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
		l.Fatalw("unable to setup firewall-controller-manager", "error", err)
	}

	// cannot use seedMgr.GetClient() because it gets initialized at a later point in time
	// we have to create an own client
	seedClient, err := controllerclient.New(seedMgr.GetConfig(), controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		l.Fatalw("unable to create seed client", "error", err)
	}

	shootAccessHelper := helper.NewShootAccessHelper(seedClient, shootAccess)
	if err != nil {
		l.Fatalw("unable to create shoot helper", "error", err)
	}

	// we do not mount the shoot client kubeconfig + token secret into the container
	// through projected token mount as the other controllers deployed by Gardener.
	//
	// the reasoning for this is:
	//
	//   - we have to pass on the shoot access to the firewall-controller, too
	//   - the firewall-controller is not a member of the Kubernetes cluster and
	//     pushing files onto the firewall is not possible
	//   - therefore, we defined flags for the shoot access generic kubeconfig and token
	//     secret for this controller and expose the access secrets through the firewall
	//     status resource, which can be read by the firewall-controller
	//   - the firewall-controller can then create a client from these secrets but
	//     it has to contiuously update the token file because the token will expire
	//   - we can re-use the same approach for this controller as well and do not have
	//     to do any additional mounts for the deployment of the controller
	//
	updater, err := helper.NewShootAccessTokenUpdater(shootAccessHelper, shootTokenPath)
	if err != nil {
		l.Fatalw("unable to create shoot access token updater", "error", err)
	}

	updater.UpdateContinuously(ctrl.Log.WithName("token-updater"), stop)

	shootConfig, err := shootAccessHelper.RESTConfig(stop)
	if err != nil {
		l.Fatalw("unable to create shoot config", "error", err)
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

	cc, err := config.New(&config.NewControllerConfig{
		SeedClient:            seedMgr.GetClient(),
		SeedConfig:            seedMgr.GetConfig(),
		SeedNamespace:         namespace,
		SeedAPIServerURL:      seedApiURL,
		ShootClient:           shootMgr.GetClient(),
		ShootConfig:           shootMgr.GetConfig(),
		ShootNamespace:        v2.FirewallShootNamespace,
		ShootAPIServerURL:     shootApiURL,
		ShootAccess:           shootAccess,
		Metal:                 mclient,
		ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
		SafetyBackoff:         safetyBackoff,
		ProgressDeadline:      progressDeadline,
		FirewallHealthTimeout: firewallHealthTimeout,
		CreateTimeout:         createTimeout,
	})
	if err != nil {
		l.Fatalw("unable to create controller config", "error", err)
	}

	if err := deployment.SetupWithManager(ctrl.Log.WithName("controllers").WithName("deployment"), seedMgr.GetEventRecorderFor("firewall-deployment-controller"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "deployment")
	}
	if err := set.SetupWithManager(ctrl.Log.WithName("controllers").WithName("set"), seedMgr.GetEventRecorderFor("firewall-set-controller"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "set")
	}
	if err := firewall.SetupWithManager(ctrl.Log.WithName("controllers").WithName("firewall"), seedMgr.GetEventRecorderFor("firewall-controller"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "firewall")
	}
	if err := monitor.SetupWithManager(ctrl.Log.WithName("controllers").WithName("firewall-monitor"), shootMgr, cc); err != nil {
		l.Fatalw("unable to setup controller", "error", err, "controller", "monitor")
	}

	if err := deployment.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "deployment")
	}
	if err := set.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "set")
	}
	if err := firewall.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		l.Fatalw("unable to setup webhook", "error", err, "controller", "firewall")
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
