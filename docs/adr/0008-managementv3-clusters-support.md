<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [8. Management V3 clusters support](#8-management-v3-clusters-support)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 8. Management V3 clusters support

* Status: proposed
* Date: 2024-01-25
* Authors: @salasberryfin
* Deciders: @richardcase @alexander-demicev @furkatgofurov7 @Danil-Grigorev @mjura

## Context

As an operator, I'd like to have an alternative import strategy that will generate a `management.cattle.io/v3` cluster instead of a `provisioning.cattle.io/v1` cluster. This is to ensure that we can support this scenario in light of future changes to Rancher.

## Decision

The current import controller logic creates a `provisioning.cattle.io/v1` cluster when a new CAPI cluster is detected. To support a new import strategy, based on a new feature flag, Rancher Turtles will either start the existing `provisioning.cattle.io` controller or a new one for `management.cattle.io`.

Adding support for this resource presents some challenges:
- Owner Reference cannot be set: `management.cattle.io/v3` clusters are cluster-scoped while CAPI clusters are namespaced.
- Display name and resource name are different.
- Cluster registration token exists in a namespace with the same name as the `management.cattle.io/v3` cluster.

Of all three, the naming strategy is the main concern. The `GenerateName` field allows us to set a prefix `c-` and automatically assigns a random name that satisfies the regular expression.

Using this random name approach, we cannot rely on `Name` and `Namespace` to be able to control the status of the cluster during reconciliation, as we do in the existing import controller. After validation, the decision is that we set labels on the Rancher clusters that make it possible to link them back to the underlying CAPI cluster. The labels are:

```
"cluster-api.cattle.io/capi-cluster-owner": name of the CAPI cluster.
"cluster-api.cattle.io/capi-cluster-owner-ns": namespace of the CAPI cluster.
```

These two labels allow us to uniquely identify each Rancher cluster, since only one CAPI cluster with a given name can exist in a given namespace. During the reconciliation process, we apply a `MatchingLabels` filter to strictly retrieve only the single cluster that contains the labels that associate it with the CAPI cluster.

From an end-user perspective:

* The default behavior remains unchanged. If no feature flag is set, the current `provisioning.cattle.io` import strategy will be used.
* User provided name will be used in Rancher dashboard while the underlying `management.cattle.io/v3` will have a name that satisfies Rancher's naming requirement: `c-` followed by 5 randomly-generated characters.

## Consequences

- Kubernetes' native garbage collector using the owner reference chain is not a viable option due to the namespaced vs global scoped conflict. This means that there needs to be a custom logic to manage deletion of resources.
- Cluster name will be used to access cluster registration token namespace.
- Display name and cluster name will be different.
