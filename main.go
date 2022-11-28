/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/go-logr/zapr"
	metalgo "github.com/metal-stack/metal-go"
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
	"github.com/metal-stack/firewall-controller-manager/controllers/set"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v2.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		logLevel              string
		metricsAddr           string
		enableLeaderElection  bool
		shootKubeconfig       string
		namespace             string
		firewallHealthTimeout time.Duration
		clusterID             string
		clusterApiURL         string
	)
	flag.StringVar(&logLevel, "log-level", "", "The log level of the controller.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":2112", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "default", "The namespace this controller is running.")
	flag.StringVar(&shootKubeconfig, "shoot-kubeconfig", "", "The path to the kubeconfig to talk to the shoot")
	flag.DurationVar(&firewallHealthTimeout, "firewall-health-timeout", 20*time.Minute, "duration after a created firewall not getting ready is considered dead")
	flag.StringVar(&clusterID, "cluster-id", "", "id of the cluster this controller is responsible for")
	flag.StringVar(&clusterApiURL, "cluster-api-url", "", "url of the cluster to put into the kubeconfi")

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

	disabledTimeout := time.Duration(-1) // wait for all runnables to finish before dying
	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      metricsAddr,
		Port:                    9443,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "firewall-controller-manager-leader-election",
		Namespace:               namespace,
		GracefulShutdownTimeout: &disabledTimeout,
	})
	if err != nil {
		setupLog.Error(err, "unable to start firewall-controller-manager")
		os.Exit(1)
	}

	shootClient := mgr.GetClient()
	if len(shootKubeconfig) > 0 {
		shootRestConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
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

	mclient, err := getMetalClient()
	if err != nil {
		setupLog.Error(err, "unable to create metal client")
		os.Exit(1)
	}

	if err = (&deployment.Reconciler{
		Seed:          mgr.GetClient(),
		Shoot:         shootClient,
		Metal:         mclient,
		Log:           ctrl.Log.WithName("controllers").WithName("deployment"),
		Namespace:     namespace,
		ClusterID:     clusterID,
		ClusterTag:    fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
		ClusterAPIURL: clusterApiURL,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "deployment")
		os.Exit(1)
	}

	if err = (&set.Reconciler{
		Seed:                  mgr.GetClient(),
		Shoot:                 shootClient,
		Metal:                 mclient,
		Log:                   ctrl.Log.WithName("controllers").WithName("set"),
		Namespace:             namespace,
		ClusterID:             clusterID,
		ClusterTag:            fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
		ClusterAPIURL:         clusterApiURL,
		FirewallHealthTimeout: firewallHealthTimeout,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "set")
		os.Exit(1)
	}

	if err = (&firewall.Reconciler{
		Seed:       mgr.GetClient(),
		Shoot:      shootClient,
		Metal:      mclient,
		Log:        ctrl.Log.WithName("controllers").WithName("firewall"),
		Namespace:  namespace,
		ClusterID:  clusterID,
		ClusterTag: fmt.Sprintf("%s=%s", tag.ClusterID, clusterID),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "firewall")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting firewall-controller-manager", "version", v.V)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running firewall-controller-manager")
		os.Exit(1)
	}
}

const (
	metalAPIUrlEnvVar = "METAL_API_URL"
	// nolint
	metalAuthTokenEnvVar   = "METAL_AUTH_TOKEN"
	metalAuthHMACEnvVar    = "METAL_AUTH_HMAC"
	metalProjectIDEnvVar   = "METAL_PROJECT_ID"
	metalPartitionIDEnvVar = "METAL_PARTITION_ID"
)

func getMetalClient() (metalgo.Client, error) {
	url := os.Getenv(metalAPIUrlEnvVar)
	token := os.Getenv(metalAuthTokenEnvVar)
	hmac := os.Getenv(metalAuthHMACEnvVar)
	projectID := os.Getenv(metalProjectIDEnvVar)
	partitionID := os.Getenv(metalPartitionIDEnvVar)

	if projectID == "" {
		return nil, fmt.Errorf("environment variable %q is required", metalProjectIDEnvVar)
	}

	if partitionID == "" {
		return nil, fmt.Errorf("environment variable %q is required", metalPartitionIDEnvVar)
	}

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
