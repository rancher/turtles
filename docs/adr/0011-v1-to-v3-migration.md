<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [11. Migration to v3 clusters](#11-migration-to-v3-clusters)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 11. Migration to v3 clusters

* Status: proposed
* Date: 2024-06-21
* Authors: @Danil-Grigorev
* Deciders: @alexander-demicev @furkatgofurov7 @salasberryfin @Danil-Grigorev @mjura @yiannistri

## Context

[ADR #8](./0008-managementv3-clusters-support.md) proposes usage of managment v3 clusters, as a resource created and managed by Turtles. This ADR extends on the idea, enabling an automatic migration path for the users still staying on v1 cluster import functionality (`provisioning.cattle.io/v1`) or users currently migrating to `v3` clusters, which is now the default.

[ADR #10](./0010-migrate-to-v3-cluster-resource.md) outlines manual migration steps, a user may want to perform to adopt rancher generated `management/v3` cluster definitions created for `provisioning/v1` clusters.

We need to provide automated procedure for the users who decide to temporarily opt-out of the `managemenv3-cluster` feature gate and cover future migration steps for them.

## Decision

Our helm chart will support automatic migration path under `managementv3-cluster-migration` feature flag.

As stated in [#8](./0008-managementv3-clusters-support.md#decision), default behaviour of cluster import is unchanged from the end user perspective. Thus we are able to automatically re-import clusters using new resource within the helm chart upgrade window. To do so, the chart has to have `post-upgrade` hook, tied to the state of the `managementv3-cluster` feature flag.

By executing migration procedure in a `post-upgrade` stage, we are making sure that the resource the hook execution will delete is no longer managed by the controller, so it does not trigger setting [deletion strategy annotation](./0003-deletion-strategy.md#decision) on the resource, and does not require additional user actions.

Hook will only execute deletion on clusters labeled with `cluster-api.cattle.io/owned` label. This label dictates ownership of the turtles controller on the resource.

Proposed behavior applies to both upgrade to `management.cattle.io/v3` cluster, or opt-out of `managementv3-cluster` scenario, migrating imported clusters back to `provisioning.cattle.io/v1` cluster.

From an end-user perspective:

* Imported clusters will loose the `-capi` suffix in the name.
* Imported clusters will shortly become unavailable, during the time the `provisioning` import manifests are removed and `management` import manifests are applied.
* Existing applications installed on the child cluster will not be removed.
* Remaining functionality stays unchanged.

## Consequences

- Turtles will no longer manage `provisioning.cattle.io/v1` clusters, and instead will work with `management.cattle.io/v3` resources.
- A feature flag `managementv3-cluster-migration` will be available to enable/disable migration behavior.
- Once the new feature gate is ready to be enabled by default, [ADR #10](./0010-migrate-to-v3-cluster-resource.md) will be deprecated in favor of the automation.
