<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [7. Rancher Turtles Public API](#7-rancher-turtles-public-api)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 7. Rancher Turtles Public API

* Status: accepted
* Date: 2023-12-04
* Authors: @Danil-Grigorev @richardcase @alexander-demicev
* Deciders: @richardcase @alexander-demicev @furkatgofurov7 @Danil-Grigorev @mjura

## Context

Rancher Turtles requires a public API implementation to facilitate provisioning and operations over `CAPI Operator` and `Cluster API` resources.

This resource should:

1. Support templating `CAPI Operator` `*Provider` manifests, including the required resource for the air-gapped environments, manager configurations and environment secrets for the provider.
1. Map `Rancher` credentials content to the `CAPI Operator` credentials secret.
1. Allow GitOps compatibility and reusability.

## Decision

The decided implementation of the API resource will be located under the `turtles-capi.cattle.io` group, kind `CAPIProvider`.

Example specification of the resource:
```yaml
apiVersion: turtles-capi.cattle.io/v1alpha1
kind: CAPIProvider
metadata:
  name: aws-infra
  namespace: default # Resource is namespace scoped.
spec:
  name: aws # Defines the provider kind to provision.
  type: infrastructure # CAPI Provider resource kind to provision.
  variables: # Additional environment variables to be declared inside the referenced spec.configSecret
    CAPA_LOGLEVEL: "4"
  configSecret:
    name: aws-credentials # This secret will be created or adjusted based on the content of the spec.features and spec.variables, without overriding non-overlapping content.
  credentials:
    rancherCloudCredential: user-credential # Rancher secret content will be translated to CAPI Operator environment secret.
  features: # Additional features to be declared inside the referenced spec.configSecret.
    clusterResourceSet: true
    clusterTopology: true
    machinePool: true
status:
  phase: Pending # Current state of the provisioning.
  variables: # Variables batch appended to the CAPI Provider secret.
    CLUSTER_TOPOLOGY: "true"
    EXP_CLUSTER_RESOURCE_SET: "true"
    EXP_MACHINE_POOL: "true"
    CAPA_LOGLEVEL: "4"
  conditions: ... # A list of conditions reflecting the current status of applied resources.
```

The `CAPIProvider` resource `CRD` will be automatically installed with the instance of the `Rancher Turtles` controller.

Each generated child resource based on the `CAPIProvider` resource spec will be owned by the resource, closely following its lifecycle, allowing the child resources to be garbage collected via ownership references.

Each templated resource will be created or updated in the cluster with the values specified in the `CAPIProvider` resource spec. `CAPIProvider.status` subresource will reflect the status of the templated resource at any moment using conditions.

GitOps compatibility is ensured for the resource, as the `.spec` subresource fields are user-managed. The provider controller is only allowed to update the `CAPIProvider.status` subresource.

Several alternatives were considered, including using HTTP-based API or using annotations on the CAPI Operator CRDs. However, the decided approach is cleaner from an implementation point of view and is aligned with the Rancher Public API strategy.

## Consequences

- Resources created and managed by the `CAPIProvider` resource will co-exist in the same namespace.
- Provider controller is only allowed to update the `.matadata` and the `.spec` (or equivalent) part of the templated resource, and the `.status` sub-resource of the `CAPIProvider` instance.