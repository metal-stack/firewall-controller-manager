package main

import (
	"context"
	"log/slog"
	"time"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	firewallDeploymentReadyReplicasDesc = prometheus.NewDesc(
		"firewall_deployment_ready_replicas",
		"provide information on firewall deployment ready replicas",
		[]string{"name", "namespace"},
		nil,
	)
	firewallDeploymentTargetReplicasDesc = prometheus.NewDesc(
		"firewall_deployment_target_replicas",
		"provide information on firewall deployment target replicas",
		[]string{"name", "namespace"},
		nil,
	)
)

type collector struct {
	log        *slog.Logger
	seedClient client.Client
	namespace  string
}

func mustRegisterCustomMetrics(log *slog.Logger, seedClient client.Client) {
	c := &collector{
		log:        log,
		seedClient: seedClient,
	}

	metrics.Registry.MustRegister(c)
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- firewallDeploymentReadyReplicasDesc
	ch <- firewallDeploymentTargetReplicasDesc
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.log.Info("collecting custom metrics")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	deploys := &v2.FirewallDeploymentList{}
	err := c.seedClient.List(ctx, deploys, client.InNamespace(c.namespace))
	if err != nil {
		c.log.Error("unable to list firewall deployments", "error", err)
		return
	}

	for _, deploy := range deploys.Items {
		ch <- prometheus.MustNewConstMetric(firewallDeploymentReadyReplicasDesc, prometheus.GaugeValue,
			float64(deploy.Status.ReadyReplicas),
			deploy.Name,
			deploy.Namespace,
		)
		ch <- prometheus.MustNewConstMetric(firewallDeploymentTargetReplicasDesc, prometheus.GaugeValue,
			float64(deploy.Status.TargetReplicas),
			deploy.Name,
			deploy.Namespace,
		)
	}
}
