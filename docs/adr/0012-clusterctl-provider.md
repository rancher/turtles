<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [12. Clusterctl Config resource](#12-clusterctl-config-resource)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 12. Clusterctl Config resource

* Status: proposed
* Date: 2024-09-03
* Authors: @Danil-Grigorev
* Deciders: @alexander-demicev @furkatgofurov7 @salasberryfin @Danil-Grigorev @mjura @yiannistri

## Context

The current implementation of clusterctl overrides is managed through `ConfigMap` deployed by turtles helm chart, which has limitations when it comes to custom providers introduction and maintenance, as it requires an update for the `ConfigMap` content, followed by `rancher/turtles` release.

The content of the `ConfigMap` is not easy to override without introducing issues, and it also makes the cluster configuration drift from the one installed with the `turtles` helm release.

## Decision

The proposed solution involves introducing a new singleton namespaced `CustomResource` - `ClusterctlConfig`, which will manage clusterctl overrides in a more flexible and maintainable way. The `ClusterctlConfig` CRD will provide `images` override options for `airGapped` installations and enable users to override or extend the default configuration with new `providers` specific to their use-case.

This additional customization layer would allow to test new provider integrations early, and will not require immediate changes to `turtles` repository, which will align it with the current [certification process](https://turtles.docs.rancher.com/tasks/provider-certification/process).

```yaml
apiVersion: turtles-capi.cattle.io/v1alpha1
kind: ClusterctlConfig
metadata:
  name: clusterctl-config # Constant name
  namespace: rancher-turtles-system # Release namaespace
spec:
  images: # https://cluster-api.sigs.k8s.io/clusterctl/configuration#image-overrides
  - name: all
    repository: myorg.io/local-repo
  providers: # https://cluster-api.sigs.k8s.io/clusterctl/configuration#provider-repositories
  - name: "my-infra-provider"
    url: "https://github.com/myorg/myrepo/releases/latest/infrastructure-components.yaml"
    type: "InfrastructureProvider"
```

The resource will manage the singleton `ConfigMap` mounted in the `turtles` `Deployment`, preserving existing functionality. The default `ConfigMap` for `clusterctl.yaml` overrides will be embedded into `turtles` release, and will serve as a default or a starting point for further modifications or customizations.

Mounted `ConfigMap` is updated automatically on a change, using [kubelet automatic mount updates](https://kubernetes.io/docs/concepts/configuration/configmap/#mounted-configmaps-are-updated-automatically).

`Turtles` will not deploy initial configuration of the resource, it will be on third-party integrations or the end user to provide customizations and deploy the resource with overrides. `Turtles` will consistently maintain the combined state of `embedded` value and user-provided config with overrides, which will be synced into mounted `ConfigMap`.

Addition of a provider to the list also allows to pin the `latest` version for the `CAPIProvider`, which references this provider `name`, to a specific value. This action prevents unexpected upgrade beyond [compatibility matrix](https://cluster-api.sigs.k8s.io/reference/versions.html?highlight%25253Dmatrix#providers-maintained-by-independent-teams) with the current `core` provider version.

## Consequences

- Turtles clusterctl `ConfigMap` will be managed by the new custom resource.
- Users will be allowed to override or extend the default configuration with values or providers specific to use-case, outside of `turtles` release cycle.