<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [13. Self managed Rancher cluster](#title)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Self managed Rancher cluster

- Status: proposed
- Date: 2025-01-20
- Authors: @anmazzotti
- Deciders: @alexander-demicev @furkatgofurov7 @salasberryfin @Danil-Grigorev @mjura @yiannistri

## Context

There are currently no best practices, documentation, or support to deploy a self managed Rancher cluster.  
In this scenario, a user such as a platform engineer starts from zero.  

- How can they bootstrap their first management Cluster?
- How to install Rancher in it?
- How to install Rancher Turtles and make the cluster also manage itself using CAPI?
- How can a self managed cluster be provisioned from an air-gapped environment?

## Decision

The proposed solution is to support a [Bootstrap & Pivot](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html#bootstrap--pivot) process, so that the user can first create a temporary management cluster, then use it to bootstrap the Rancher management cluster, and finally pivot all needed resources to it.  

In order to ensure compatibility and sane validation checks, initing and moving resources should be done using the [clusteroperator](https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/cmd/plugin/cmd) CLI.  
An additional reason to use `clusteroperator` is to make use of the upcoming `preload` [support](https://github.com/kubernetes-sigs/cluster-api-operator/pull/683) and being able to load manifests from OCI images, which will ease the air-gapped scenarios.  

A simplified sequence of actions should look like:

1. Air-gap preparation steps if needed
1. Deploy a temporary cluster
1. `clusteroperator init ...`
1. Provision a downstream `rancher` CAPI cluster
1. Initialize the cluster installing Rancher & Turtles (with all needed providers)
1. `clusteroperator move rancher ...`
1. Delete the temporary cluster

Support for a self managed cluster can be improved in different iterations:

1. Provide and document a sample script to deploy a self managed Rancher cluster on CAPD+RKE2 using kind
1. Support air-gapped scenario (for kind --> CAPD+RKE2 sample)
1. Implement and use a test suite to deploy self managed Rancher clusters on all supported providers
1. Test scenarios should support upgrading providers on the self managed cluster, and upgrading the k8s version too, to ensure a correct cluster lifecycle
1. Document how to use the test suite to automate self managed cluster provisioning for end-users

## Consequences

- `clusteroperator move` needs to be implemented first
- Air-gapped scenarios are different depending on the infrastructure providers used, wil need to pay attention to not overlap with provider specific air-gap logic or setup instructions
- There is no clear cluster-api contract for self-managing clusters. This should not be a problem by design, but it could be improved, for example by preventing self-managed cluster accidental deletion.
- Different ways of deploying the temporary cluster can be supported. One notable example being Rancher Desktop, to make the process accessible to most users.
