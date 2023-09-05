<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [4. Running out of Rancher Manager cluster](#4-running-out-of-rancher-manager-cluster)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 4. Running out of Rancher Manager cluster

* Status: proposed
* Date: 2023-08-31
* Authors: @salasberryfin
* Deciders: @richardcase @alexander-demicev @furkatgofurov7 @Danil-Grigorev @mjura

## Context

As an operator, I want the choice to deploy Rancher Turtles in the same or a 
different cluster to Rancher Manager, so that I have a choice in my deployment 
topology.

## Decision

`CAPIImportReconciler` will contain two differentiated clients:
- `Client`: in-cluster client for the controller and the CAPI resources.
- `RancherClient`: for the Rancher Manager cluster.

A flag `rancher-kubeconfig` with a path to a kubeconfig file is used to select 
the type of installation:
- If no value is passed, Rancher Turtles is set to the same cluster 
installation: `Client` and `RancherClient` are the same instance of the client.
- If a valid path to a kubeconfig file is passed, `RancherClient` is set to 
client created from the kubeconfig.

From an end-user perspective:

* No parameter is required if Rancher Manager and Rancher Turtles run in the 
same cluster.
* User will provide a path to a valid kubeconfig file that is available in the 
pod if installing Rancher Turtles outside of Rancher Manager cluster.

## Consequences

- The path to the kubeconfig must be mounted in the pod to be accessible. This 
pattern is aligned with what is used in [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime/blob/964a416cf5acf5a67bdc5904897006447ac11509/pkg/client/config/config.go#L60).
