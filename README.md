# firewall-controller-manager

## Overview

The firewall-controller-manager aka FCM is a collection of controllers which are responsible for managing the lifecycle of metal-stack firewalls in a bare-metal kubernetes cluster. It is roughly inspired by the design of Gardener's Machine Controller Manager and Kubernetes' built-in resources `Deployment`, `ReplicaSet` and `Pod`.

## Objects

There are the following similarities in the resources of FCM compared to a Deployment:

| Custom ResourceObject | Description                                                                                                                                                   |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FirewallDeployment`  | A `FirewallDeployment` contains the spec template of a `Firewall` resource similar to a `Deployment` and implements update strategies like rolling update.    |
| `FirewallSet`         | A `FirewallSet` is similar to ReplicaSet. It is typically owned by a `FirewallDeployment` and attempts to run the defined replica amount of the `Firewall`(s) |
| `Firewall`            | A `Firewall` is similar to a `Pod` and has a 1:1 relationship to a firewall in the metal-stack api.                                                           |

If significant changes were made to the `FirewallDeployment` – like changing the OS image, machine size or firewall networks – then a new `FirewallSet` is created and the existing `Firewall` will be eventually replaced.

The way how a `Firewall` is replaced can be defined with the `FirewallUpdateStrategy`.

## Architecture

There are three controllers implemented with the following responsibilities.

### `FirewallDeploymentController`

Reconciles the `FirewallDeployment` which was created and manages the lifecycle of a `FirewallSet`. It creates a ServiceAccount Token for the firewall to be able to talk to the kubernetes-api server. The template spec is validated and if changes were made, it decides if a new `FirewallSet` must be created or if the existing one just needs to be updated. The resource status shows the overall status.

### `FirewallSetController`

Creates and deletes `Firewall` objects according to the spec according to the given number of firewall replicas. It also checks the status of the `Firewall` and report that in the own status.

### `FirewallController`

Create and delete the physical firewall machine from the spec at the metal-api.

## Deployment

Firewall Controller Manager must be deployed into the Shoot Namespace in a Seed Cluster if this is a Gardener Managed environment. So the Gardener Extension Provider Metal have to create a appropriate Deployment.

TODO: Create deployment.
