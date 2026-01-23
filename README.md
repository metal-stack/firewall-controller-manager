# firewall-controller-manager

## Overview

The firewall-controller-manager (FCM) is a collection of controllers which are responsible for managing the lifecycle of firewalls in a [Gardener](https://gardener.cloud/) shoot cluster for the metal-stack provider.

The FCM is typically deployed into the shoot namespace of a seed cluster. This is done by the [gardener-extension-provider-metal](https://github.com/metal-stack/gardener-extension-provider-metal/).

The design of the FCM is roughly inspired by Gardener's [machine-controller-manager](https://github.com/gardener/machine-controller-manager) and Kubernetes' built-in resources `Deployment`, `ReplicaSet` and `Pod`.

## Architecture

The following table is a summary over the [CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) introduced by the FCM:

| Custom Resource Object | Description                                                                                                                                                     |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FirewallDeployment`   | A `FirewallDeployment` contains the spec template of a `Firewall` resource similar to a `Deployment` and implements update strategies like rolling update.      |
| `FirewallSet`          | A `FirewallSet` is similar to `ReplicaSet`. It is typically owned by a `FirewallDeployment` and attempts to run the defined replica amount of the `Firewall`(s) |
| `Firewall`             | A `Firewall` is similar to a `Pod` and has a 1:1 relationship to a firewall in the metal-stack api.                                                             |
| `FirewallMonitor`      | Deployed into the cluster of the user (shoot cluster), which is useful for monitoring the firewall or user-triggered actions on the firewall.                   |

### `FirewallDeploymentController`

The `FirewallDeployment` controller manages the lifecycle of `FirewallSet`s. It syncs the `Firewall` template spec and if significant changes were made, it may trigger a `FirewallSet` roll. When choosing `RollingUpdate` as a deployment strategy, the deployment controller is waiting for the firewall-controller to connect before throwing away an old `FirewallSet`. The `Recreate` strategy first releases firewalls before creating a new one (can be useful for environments which ran out of available machines but you still want to update).

The controller also deploys a service account for the firewall-controller to be able to talk to the seed's kube-apiserver.

### `FirewallSetController`

Creates and deletes `Firewall` objects according to the spec and the given number of firewall replicas. It also checks the status of the `Firewall` and report that in the own status.

### `FirewallController`

Creates and deletes the physical firewall machine from the spec at the [metal-api](https://github.com/metal-stack/metal-api).

## Rolling a `FirewallSet` through `FirewallMonitor` Annotation

A user can initiate rolling the latest firewall set by annotating a monitor in the following way:

```bash
kubectl annotate fwmon <firewall-name> firewall.metal-stack.io/roll-set=true
```

## Development

Most of the functionality is developed with the help of the [integration](integration) test suite.

To play with the FCM, you can also run this controller inside the [mini-lab](https://github.com/metal-stack/mini-lab) and without a running Gardener installation:

1. Start up the mini-lab, run `eval $(make dev-env)` and change back to this project's directory
1. Deploy the FCM into the mini-lab with `make deploy`
1. Adapt the example [firewalldeployment.yaml](config/examples/firewalldeployment.yaml) and apply with `kubectl apply -f config/examples/firewalldeployment.yaml`
1. Note that the firewall-controller will not be able to connect to the mini-lab due to network restrictions, so the firewall will not get ready.
   - You can make the firewall become ready anyway by setting the annotation `kubectl annotate fw <fw-nsme> firewall.metal-stack.io/no-controller-connection=true`
