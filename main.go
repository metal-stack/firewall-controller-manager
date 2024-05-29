package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"

	metalgo "github.com/metal-stack/metal-go"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-lib/pkg/tag"
	"github.com/metal-stack/v"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/firewall-controller-manager/api/v2/helper"
	"github.com/metal-stack/firewall-controller-manager/controllers"
	"github.com/metal-stack/firewall-controller-manager/controllers/deployment"
	"github.com/metal-stack/firewall-controller-manager/controllers/firewall"
	"github.com/metal-stack/firewall-controller-manager/controllers/monitor"
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	"github.com/metal-stack/firewall-controller-manager/controllers/update"
)

const (
	metalAuthHMACEnvVar = "METAL_AUTH_HMAC"
)

func healthCheckFunc(log *slog.Logger, seedClient controllerclient.Client, namespace string) func(req *http.Request) error {
	return func(req *http.Request) error {
		log.Debug("health check called")

		fws := &v2.FirewallList{}
		err := seedClient.List(req.Context(), fws, controllerclient.InNamespace(namespace))
		if err != nil {
			return fmt.Errorf("unable to list firewalls in namespace %s", namespace)
		}
		return nil
	}
}

func main() {
	var (
		scheme   = helper.MustNewFirewallScheme()
		metalURL string

		logLevel                string
		metricsAddr             string
		healthAddr              string
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
		internalShootApiURL     string
		seedApiURL              string
		certDir                 string
	)

	flag.StringVar(&logLevel, "log-level", "info", "the log level of the controller")
	flag.StringVar(&metricsAddr, "metrics-addr", ":2112", "the address the metric endpoint binds to")
	flag.StringVar(&healthAddr, "health-addr", ":8081", "the address the health endpoint binds to")
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
	flag.StringVar(&shootApiURL, "shoot-api-url", "", "url of the shoot api server, if not provided falls back to single-cluster mode")
	flag.StringVar(&internalShootApiURL, "internal-shoot-api-url", "", "url of the shoot api server used by this controller, not published in the shoot access status")
	flag.StringVar(&seedApiURL, "seed-api-url", "", "url of the seed api server")
	flag.StringVar(&certDir, "cert-dir", "", "the directory that contains the server key and certificate for the webhook server")
	flag.StringVar(&shootKubeconfigSecret, "shoot-kubeconfig-secret-name", "", "the secret name of the generic kubeconfig for shoot access")
	flag.StringVar(&shootTokenSecret, "shoot-token-secret-name", "", "the secret name of the token for shoot access")
	flag.StringVar(&sshKeySecret, "ssh-key-secret-name", "", "the secret name of the ssh key for machine access")
	flag.StringVar(&shootTokenPath, "shoot-token-path", "", "the path where to store the token file for shoot access")

	flag.Parse()

	slogHandler, err := controllers.NewLogger(logLevel)
	if err != nil {
		ctrl.Log.WithName("setup").Error(err, "unable to parse log level")
		os.Exit(1)
	}
	l := slog.New(slogHandler)

	ctrl.SetLogger(logr.FromSlogHandler(slogHandler))

	var (
		stop = ctrl.SetupSignalHandler()
	)

	mclient, err := getMetalClient(metalURL)
	if err != nil {
		log.Fatalf("unable to create metal client %v", err)
	}

	seedMgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: certDir,
		}),
		Cache: cache.Options{
			SyncPeriod: &reconcileInterval,
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
		},
		HealthProbeBindAddress:  healthAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "firewall-controller-manager-leader-election",
		GracefulShutdownTimeout: &gracefulShutdownTimeout,
	})
	if err != nil {
		log.Fatalf("unable to setup firewall-controller-manager %v", err)
	}

	// cannot use seedMgr.GetClient() because it gets initialized at a later point in time
	// we have to create an own client
	seedClient, err := controllerclient.New(seedMgr.GetConfig(), controllerclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Fatalf("unable to create seed client %v", err)
	}

	if err := seedMgr.AddHealthzCheck("health", healthCheckFunc(l.WithGroup("health"), seedClient, namespace)); err != nil {
		log.Fatalf("unable to set up health check %v", err)
	}
	if err := seedMgr.AddReadyzCheck("check", healthCheckFunc(l.WithGroup("ready"), seedMgr.GetClient(), namespace)); err != nil {
		log.Fatalf("unable to set up ready check %v", err)
	}

	var (
		externalShootAccess = &v2.ShootAccess{
			GenericKubeconfigSecretName: shootKubeconfigSecret,
			TokenSecretName:             shootTokenSecret,
			Namespace:                   namespace,
			APIServerURL:                shootApiURL,
		}
		internalShootAccess       = externalShootAccess.DeepCopy()
		internalShootAccessHelper *helper.ShootAccessHelper
	)

	if internalShootApiURL != "" {
		internalShootAccess.APIServerURL = internalShootApiURL
	}

	if shootApiURL == "" {
		shootApiURL = seedMgr.GetConfig().Host

		internalShootAccessHelper = helper.NewSingleClusterModeHelper(seedMgr.GetConfig())
		if err != nil {
			log.Fatalf("unable to create shoot helper %v", err)
		}
		l.Info("running in single-cluster mode")
	} else {
		internalShootAccessHelper = helper.NewShootAccessHelper(seedClient, internalShootAccess)
		if err != nil {
			log.Fatalf("unable to create shoot helper %v", err)
		}
		l.Info("running in split-cluster mode (seed and shoot client)")
	}

	if shootTokenPath != "" {
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
		updater, err := helper.NewShootAccessTokenUpdater(internalShootAccessHelper, shootTokenPath)
		if err != nil {
			log.Fatalf("unable to create shoot access token updater %v", err)
		}

		err = updater.UpdateContinuously(ctrl.Log.WithName("token-updater"), stop)
		if err != nil {
			log.Fatalf("unable to start token updater %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shootConfig, err := internalShootAccessHelper.RESTConfig(ctx)
	if err != nil {
		log.Fatalf("unable to create shoot config %v", err)
	}

	shootMgr, err := ctrl.NewManager(shootConfig, ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: "0",
		},
		LeaderElection: false,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				v2.FirewallShootNamespace: {},
			},
		},
		GracefulShutdownTimeout: pointer.Pointer(time.Duration(0)),
	})
	if err != nil {
		log.Fatalf("unable to start firewall-controller-manager-monitor %v", err)
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
		ShootAccess:           externalShootAccess,
		SSHKeySecretName:      sshKeySecret,
		SSHKeySecretNamespace: namespace,
		ShootAccessHelper:     internalShootAccessHelper,
		Metal:                 mclient,
		ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
		SafetyBackoff:         safetyBackoff,
		ProgressDeadline:      progressDeadline,
		FirewallHealthTimeout: firewallHealthTimeout,
		CreateTimeout:         createTimeout,
	})
	if err != nil {
		log.Fatalf("unable to create controller config %v", err)
	}

	if err := deployment.SetupWithManager(ctrl.Log.WithName("controllers").WithName("deployment"), seedMgr.GetEventRecorderFor("firewall-deployment-controller"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup deployment controller: %v", err)
	}
	if err := set.SetupWithManager(ctrl.Log.WithName("controllers").WithName("set"), seedMgr.GetEventRecorderFor("firewall-set-controller"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup set controller: %v", err)
	}
	if err := firewall.SetupWithManager(ctrl.Log.WithName("controllers").WithName("firewall"), seedMgr.GetEventRecorderFor("firewall-controller"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup firewall controller: %v", err)
	}
	if err := monitor.SetupWithManager(ctrl.Log.WithName("controllers").WithName("firewall-monitor"), shootMgr, cc); err != nil {
		log.Fatalf("unable to setup monitor controller: %v", err)
	}
	if err := update.SetupWithManager(ctrl.Log.WithName("controllers").WithName("update"), seedMgr.GetEventRecorderFor("update-controller"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup update controller: %v", err)
	}

	if err := deployment.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup webhook, controller deployment %v", err)
	}
	if err := set.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup webhook, controller set %v", err)
	}
	if err := firewall.SetupWebhookWithManager(ctrl.Log.WithName("defaulting-webhook"), seedMgr, cc); err != nil {
		log.Fatalf("unable to setup webhook, controller firewall %v", err)
	}

	go func() {
		l.Info("starting shoot controller", "version", v.V)
		if err := shootMgr.Start(stop); err != nil {
			log.Fatalf("problem running shoot controller %v", err)
		}
	}()

	l.Info("starting seed controller", "version", v.V)
	if err := seedMgr.Start(stop); err != nil {
		log.Fatalf("problem running seed controller %v", err)
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
