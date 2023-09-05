<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [5. Cluster Import Strategy](#5-cluster-import-strategy)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 5. Cluster Import Strategy
<!-- A short and clear title which is prefixed with the ADR number -->

- Status: proposed
- Date: 2023-09-04
- Authors: @richardcase
- Deciders: @richardcase

## Context

Clusters that are provisioned via Cluster API (CAPI) will need to be made available via Rancher Manager. There is a feature of Rancher Manager that allows you to **import** an existing cluster.

## Decision

The operator will replicate the steps that a user would do if they where importing an existing cluster via the Rancher Manager UI including applying the manifests available via the import endpoint for the newly imported cluster.

Its also been decided that we will not use the import feature of v2prov as this will restrict us to always having the CAPI Management be the same as the Rancher Manager Cluster. See [ADR004](0004-running-out-of-rancher-manager-cluster.md) for further details of running the operator in a different cluster.

## Consequences

To replicate the import steps the operator will need to do:

- Create an instance of **provisioning.cattle/v1.Cluster** in the Rancher Manager cluster
- Wait for the **ClusterRegistrationToken** to be created AND contant an import url
- Download the import manifests using the URL from the **ClusterRegistrationToken**
- Use the kubeconfig generate by CAPI for the cluster and apply the import manifests to cluster created by CAPI.
- This will deploy the Rancher Cluster agent which will then connect back to Rancher Manager
