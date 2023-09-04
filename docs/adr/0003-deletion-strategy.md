<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [3. Deletion strategy](#3-deletion-strategy)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 3. Deletion strategy

* Status: proposed
* Date: 2023-08-22
* Authors: @furkatgofurov7
* Deciders: @richardcase @alexander-demicev @salasberryfin @Danil-Grigorev @mjura

## Context

As operator, it is crucial to establish clear guidelines for how Rancher turtles handle cluster deletion operations. The objective is to facilitate the removal of a CAPI-imported cluster from Rancher, ensuring that it no longer appears in the Rancher UI, all while keeping the underlying CAPI cluster intact.

## Decision

The deletion workflow is structured as follows:

**Ownership Chain:** Rancher cluster is associated with CAPI cluster (CAPI cluster owns the Rancher cluster) through the owner references chain during their creation process.

**Cluster Annotation:** When deleting a Rancher cluster, the operator will annotate the corresponding CAPI cluster with the `ClusterImportedAnnotation` (`imported=“true”`) annotation. This annotation will prevent automatic re-import of the CAPI cluster after corresponding Rancher cluster deletion. The underlying infrastructure provisioned by CAPI is left intact.

From an end-user perspective:

* If user manually deletes CAPI cluster. Rancher turtles uses [Kubernetes owner references](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/) to track relationships between objects. These references are used for Kubernetes garbage collection, which is the basis of Rancher cluster deletion in Rancher turtles.
* If user manually removes the Rancher cluster from UI: depending on the `imported` annotation presence in the CAPI cluster, operator blocks Rancher cluster from being re-imported (re-created) after deletion. 

## Consequences

- The operator will leverage Kubernetes' native garbage collection mechanism, by using the owner reference chain, to ensure a streamlined cluster deletion process.
- operator will be able to perform a selective deletion in a controlled manner by using annotations.