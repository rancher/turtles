<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [9. Use structured proxy types](#9-use-structured-proxy-types)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 9. Use structured proxy types

- Status: proposed
- Date: 2023-08-09
- Authors: @Danil-Grigorev
- Deciders: @richardcase @alexander-demicev @salasberryfin @furkatgofurov7 @mjura

## Context

As described in `Rancher` integration strategy [ADR](./0005-rancher-integration-strategy.md#context) we'd like to operate on several `CRs` provided by the `Rancher`. Although the use of unstructured objects may be avoided by specifying a subset definition of the `CRs` we are using, without a need to import `Rancher` API packages directly.

## Decision

The `rancher-turtles` operator will be using `kubebuilder` [annotations](https://kubebuilder.io/reference/markers) on specified for the `Rancher` proxy types located under the `./internal/rancher` directory, to generate deep copy definitions and therefore allow specified resources to match the `Object` [interface](https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/client/object.go#L45) provided by the controller-runtime.

This will allow us to:

1. Generate independed definitions for `rancher` custom resources, allowing them to function independently from `Rancher` types without losing the functionality provided by the `Object` interface.
2. Utilize the `client.Object` interface and helper functions provided for it.
3. Avoid converting objects into unstructured types and back, reducing unnecessary processing overhead or chance of setting unknown API fields on these objects.
4. Use of `envtest` instead of `fakeClient`. With this setup, this can employ regular [setup procedures](https://kubebuilder.io/reference/envtest.html?highlight=envtest#writing-tests) and utilize `envtest` in testing rather than dealing with the complications that come with the `fakeClient` (see the [issue](https://github.com/kubernetes-sigs/controller-runtime/issues/2308)). This simplifies writing integration tests and ensures a more straightforward testing process.

Since we have direct control over their resource specifications when working with proxied object types, we can maintain consistency with the latest available `Rancher` API versions.

## Consequences

- We no longer use **unstructured.Unstructured** when creating, watching, listing and reading resources using controller-runtime, instead we register our proxy `Rancher` API types in the schema builder directly on operator startup.
- We will test our code using `envtest`.
