<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [5. Rancher Integration Strategy](#5-rancher-integration-strategy)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 5. Rancher Integration Strategy

- Status: accepted
- Date: 2023-06-01
- Authors: @richardcase
- Deciders: @richardcase

## Context

The Rancher Turtles operator (a.k.a Rancher CAPI Extension) needs to integrate with Rancher Manager. The operator needs to perform the steps that a user would do when importing a cluster via the Rancher Manager UI. This means the operator needs to:

- create & delete instances of the **provisioning.cattle/v1.Cluster** custom resource.
- Read instances of the **ClusterRegistrationToken** custom resource.

## Decision

Both the custom resources are defined in the **github.com/rancher/rancher/pkg/apis** Go package of Rancher Manager.  We considered importing this package directly and using the API definitions directly. However, due to the use of a fork of **client-go** by that package it would make our dependency management difficult and tie the operator to specific versions of many of the Kubernetes packages.

The decision is that we will use **unstructured.Unstructured** so that we don't have to depend of the Rancher Manager apis package. This will make the dependency management easier and will allow us to use different versions of the core Kubernetes packages (like **client-go**).

The downside is that **unstructured.Unstructured** is essentially untyped (its a map of interface{}) so we need to be very careful that we construct the resources correctly, with the correct fields and data types when creating instances of custom resources. Likewise, we need to ensure we navigate the structure correctly when we read instances of the custom resources.

## Consequences

Adopting the approach means we will need to do the following:

- Use **unstructured.Unstructured** when creating, watching, listing and reading resources using controller-runtime
- Ensure the **github.com/rancher/rancher/pkg/apis** package isn't imported
- Investigate options to encapsulate the use of **unstructured.Unstructured** so that the logic of the operator deals with a normal struct
