# How to bump core CAPI in Turtles

## Overview

This guide explains how to bump the core Cluster API (CAPI) version in `rancher/turtles`.

A complete bump usually includes:

- Bumping Go modules.
- Publishing the downstream image.
- Fetching the core CAPI manifest and embedding it into the Helm chart.
- Updating image references in both:
  - `rancher/turtles`
  - `rancher/rancher` for image mirroring.

## What changes

## 1. Choose the target version

Before changing anything, decide:

- the target core CAPI release, for example `v1.x.y`;
- the downstream image tag and registry path expected by Rancher/Turtles.

Keep in mind:

- Turtles code depends on the following Go modules:
  - `sigs.k8s.io/cluster-api`.
  - `sigs.k8s.io/cluster-api/test`.
- the embedded manifest is fetched from Rancher’s Cluster API release artifacts
(downstream release `rancher/cluster-api`).

Make sure:

- The upstream Go module version exists.
- The Rancher release artifact for `core-components.yaml` exists and the
downstream image is available. If it does not exist yet, publish a new
downstream release first (see [Create downstream release](#2-create-downstream-release)).

## 2. Create downstream release

The repository [rancher/clusterapi-forks](https://github.com/rancher/clusterapi-forks)
is the orchestrator for publishing downstream CAPI (and provider) releases.

Each forked CAPI project has its corresponding GitHub Action in
`rancher/clusterapi-forks` for creating a new downstream release. Additionally,
a periodically executed workflow [syncs the forks](https://github.com/rancher/clusterapi-forks/actions/workflows/repo-sync.yaml).

To create a new core CAPI release, navigate to the [Core](https://github.com/rancher/clusterapi-forks/actions/workflows/core.yaml)
action and trigger a new workflow run selecting the desired CAPI version.

After a successful execution of the GitHub Action, a new release for the desired
CAPI version should be available in [rancher/cluster-api](https://github.com/rancher/cluster-api).

## 3. Bump Go modules

Update the root module and test module and run `make generate` to update dependencies.

Note: CAPI Go modules and CAPI controller releases are technically not related,
and using different versions may be compatible, specially if minor version is a
match. However, **as a good practice we should try to align controller and Go
module versions**.

## 4. Fetch core CAPI manifest

To simplify the air-gapped experience, the core CAPI manifest installed with
Turtles is not dynamically fetched from remote. Instead, the yaml manifest is
embedded in the `rancher-turtles` Helm chart and CAPI Operator installs it from
a `ConfigMap`.

This means that each time core CAPI is bumped, the embedded yaml must be
retrieved and embedded, for which `rancher/turtles` provides a simple process
that you can execute by just running (replace with your desired version):

```bash
CAPI_MANIFEST_UPDATE_VERSION=v1.13.1 make update-core-capi-manifest
```

This will automatically update the Helm chart with the CAPI manifest.

## 5. Update core CAPI controller image references

Since the core CAPI image tag is referenced in multiple `rancher/turtles` files,
the **simplest solution** is to search for all occurrences of the current
version and replace it with the new one.

Finally, there's an extra reference to the core CAPI image version in
[rancher/rancher](https://github.com/salasberryfin/rancher/blob/4d8e0e669706c996fbedefdf81b0dd351b4ceb6b/scripts/package-env#L14).

It is very important that you don't forget to update this entry in
`rancher/rancher`, as it will be the source for mirroring the image when Rancher
is released. Otherwise, it will be missing from the required SUSE registries and
it will create a release-blocker bug.

Simply update the following line with the version that Turtles has been bumped
to:

```bash
CLUSTER_API_CONTROLLER_TAG=v1.13.1
```

## Post update steps

If the new version of CAPI includes relevant code changes, e.g. new API version,
the full end-to-end test suite should be verified, as not all errors may show in
the PRs CI checks.

To fully verify the update, the following is recommended:

1. Create an upstream branch and push the changes.
1. Open a draft PR.
1. Run [Long E2E](https://github.com/rancher/turtles/actions/workflows/e2e-long.yaml)
and [vSphere E2E](https://github.com/rancher/turtles/actions/workflows/run-vsphere-tests.yaml).
1. Mark PR ready for review linking the successful end-to-end executions.
