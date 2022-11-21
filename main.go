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
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/go-logr/zapr"
	"github.com/metal-stack/v"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/metal-stack/firewall-controller-manager/controllers"
	firewallcontrollerv1 "github.com/metal-stack/firewall-controller/api/v1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(firewallcontrollerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		logLevel             string
		metricsAddr          string
		enableLeaderElection bool
		shootKubeconfig      string
		namespace            string
	)
	flag.StringVar(&logLevel, "log-level", "", "The log level of the controller.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", "default", "The namespace this controller is running.")
	flag.StringVar(&shootKubeconfig, "shoot-kubeconfig", "", "The path to the kubeconfig to talk to the shoot")

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

	if err = (&controllers.Reconciler{
		Seed:      mgr.GetClient(),
		Shoot:     shootClient,
		Log:       ctrl.Log.WithName("controllers").WithName("firewall"),
		Namespace: namespace,
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
