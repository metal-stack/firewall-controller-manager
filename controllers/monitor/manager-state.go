package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/metal-stack/firewall-controller-manager/api/v2/config"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type deploymentRef struct {
	namespace string
	name      string
}

func deploymentRefTo(deploy *v2.FirewallDeployment) deploymentRef {
	return deploymentRef{
		namespace: deploy.Namespace,
		name:      deploy.Name,
	}
}

type MonitorManagerScheduler struct {
	m        sync.RWMutex
	managers map[deploymentRef]context.CancelFunc

	scheme *runtime.Scheme
	log    logr.Logger
	cc     *config.NewControllerConfig
	c      *config.ControllerConfig
}

func NewMonitorManagerState(log logr.Logger, c *config.ControllerConfig) *MonitorManagerScheduler {
	return &MonitorManagerScheduler{
		managers: make(map[deploymentRef]context.CancelFunc),
	}
}

func (m *MonitorManagerScheduler) StartIfNeeded(ctx context.Context, deploy *v2.FirewallDeployment) error {
	m.m.Lock()
	defer m.m.Unlock()

	if _, exists := m.managers[deploymentRefTo(deploy)]; exists {
		return nil
	}

	ref := deploymentRefTo(deploy)
	log := m.log.WithValues("namespace", ref.namespace, "name", ref.name)

	shootConfig, err := m.cc.ShootAccessHelper.RESTConfig(ctx) // TODO: adjust to fetch the kubeconfig
	if err != nil {
		return fmt.Errorf("unable to get shoot rest config %w", err)
	}

	shootMgr, err := ctrl.NewManager(shootConfig, ctrl.Options{
		Scheme: m.scheme,
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
		return fmt.Errorf("unable to start firewall-controller-manager-monitor %w", err)
	}

	err = SetupWithManager(log, shootMgr, m.c)
	if err != nil {
		return fmt.Errorf("unable to setup firewall-controller-manager-monitor %w", err)
	}

	shootCtx, cancel := context.WithCancel(ctx)

	err = shootMgr.Start(shootCtx)
	if err != nil {
		cancel()
		return fmt.Errorf("unable to start firewall-controller-manager-monitor %w", err)
	}

	m.managers[ref] = cancel
	return nil
}

func (m *MonitorManagerScheduler) Stop(deploy *v2.FirewallDeployment) {
	m.m.Lock()
	defer m.m.Unlock()

	cancel, exists := m.managers[deploymentRefTo(deploy)]
	if exists {
		cancel()
		delete(m.managers, deploymentRefTo(deploy))
	}
}
