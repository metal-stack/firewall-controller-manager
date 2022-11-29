# firewall-controller-manager

## Overview

The Firewall Controller Manager aka FCM is a collection of controllers which are responsible manage the livecycle of firewall(s) of a bare-metal kubernetes cluster. It is roughly inspired by the design of the Machine Controller Manager and the reconcile principles of Deployment, Replicaset and Pod.

## Objects

There are the following similarities in the resources of FCM compared to a Deployment:

| Custom ResourceObject | Description                                                                                                                             |
|-----------------------|-----------------------------------------------------------------------------------------------------------------------------------------|
| `FirewallDeployment`  | A `FirewallDeployment` contains the definition of a Firewall Object to be created similar to a `Deployment`                             |
| `FirewallSet`         | A `FirewallSet` is similar to Replicaset, is created for a `FirewallDeployment` and defines the `Firewall`(s) which needs to be created |
| `Firewall`            | A `Firewall` is the end result, similar to a `Pod`                                                                                      |

If significant changes where made to the `FirewallDeployment`, like Image, Size, Networks have changed, then a new `FirewallSet` is created and the existing `Firewall` will be eventually replaced.

The way how a `Firewall` is replaced can be defined with the `FirewallUpdateStrategy`.

## Architecture

There are 3 controllers implemented with the following responsibilities.

### FirewallDeploymentController

Reconciles the `FirewallDeployment` which was created and manages the livecycle of a `FirewallSet`. It creates a ServiceAccount Token for the firewall to be able to talk to the kubernetes-api server. The Spec is validated and if changes where made, it decides if a new `FirewallSet` must be created and delete the old `FirewallSet`. The Status shows the overall status.

### FirewallSetController

Creates and deletes `Firewall` Objects according to the Spec. Also checks the Status of the `Firewall` and report that in the own Status.

### FirewallController

Create and delete the physical Firewall Machine from the `Firewall.Spec`.

## Deployment

Firewall Controller Manager must be deployed into the Shoot Namespace in a Seed Cluster if this is a Gardener Managed environment. So the Gardener Extension Provider Metal have to create a appropriate Deployment.

TODO: Create deployment.