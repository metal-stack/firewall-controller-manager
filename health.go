package main

import (
	"fmt"
	"log/slog"
	"net/http"

	v2 "github.com/metal-stack/firewall-controller-manager/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func healthCheckFunc(log *slog.Logger, seedClient client.Client, namespace string) func(req *http.Request) error {
	return func(req *http.Request) error {
		log.Debug("health check called")

		fws := &v2.FirewallList{}
		err := seedClient.List(req.Context(), fws, client.InNamespace(namespace))
		if err != nil {
			return fmt.Errorf("unable to list firewalls in namespace %s", namespace)
		}
		return nil
	}
}
