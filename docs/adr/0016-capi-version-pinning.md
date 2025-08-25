<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [CAPI version pinning strategy (community vs prime)](#capi-version-pinning-strategy-community-vs-prime)
  - [Context](#context)
  - [Options](#options)
  - [Decision](#decision)
  - [Open questions](#open-questions)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# CAPI version pinning strategy (community vs prime)

- Status: proposed
- Date: 2025-08-25
- Authors: [@alexander-demicev]
- Deciders: [TBD]

## Context

We need a clear strategy for pinning the Core CAPI version and CAPI provider versions across community and prime builds. Today, Turtles embeds a `clusterctl` configuration as `config.yaml` (see `internal/controllers/clusterctl/config.yaml`, embedded via `internal/controllers/clusterctl/config.go`). The question is where those pins should live and how we differentiate community vs prime delivery.

## Options

1) Keep existing `config.yaml`, switch by build tag
- Keep an embedded `config.yaml` but provide a Go build tag to select the correct version:
  - Prime version: pin Core CAPI and providers to our forks.
  - Community version: pin only Core CAPI to upstream.
- Pros: possbily smallest change out of all options, easy to configure.
- Cons: difficult to explain and document how it's combined with ClusterctlConfig.

2) No provider pinning in Turtles, use separate providers Helm chart with [ClusterctlConfig](https://turtles.docs.rancher.com/turtles/stable/en/reference/clusterctlconfig.html)
- Do not hardcode provider pins in Turtles. Create a separate Helm chart for providers and do the pinning there via `ClusterctlConfig` resource. For prime users, make this chart mandatory to use CAPI providers. Community users are on their own to manage provider versions.
- Pros: clean separation, providers can be updated independently of Turtles code.
- Cons: introduces a strongly required dependency for provider pinning functionality, still need to figure out core CAPI pinning for community.

3) Create two Turtles charts (community and prime), move pinning entirely to these charts
- Do not embed `config.yaml` in Turtles. Provide two Helm charts:
  - Community chart with a `ClusterctlConfig` where core CAPI is pinned to upstream.
  - Prime chart with a `ClusterctlConfig` pinned to Rancher forks.
- Pros: even cleaner separation, providers can be updated independently of Turtles code.
- Cons: more packaging/release complexity, users must adopt charts for installation.

4) Create one Turtles chart (community && prime) and one Turtles Certified Providers chart (prime only). 
- Core CAPI provider is installed by the Turtles chart and always pinned for all users. 
- Core CAPI provider uses the upstream ghcr.io registry by default, however this can be configured as a chart value, so when Rancher installs the Turtles chart, it can override the image to registry.suse.com (and configure the chart for Prime in other ways via values)
- Turtles Certified Providers chart can be only published for prime users. It can load all manifests via oci, it will have all versions pinned. Community users can pin their own versions using CAPIProvider. 
- Pros: It's one build, both for Turtles binary and for the Turtles chart. No shenanigans, no hardcoded embedded configuration values. Everything is configured at the chart level, fully transparent to the users.
- Cons: Introduces complexity with multiple charts, requiring users to install and manage additional components beyond a simple CAPIProvider CR. On Rancher updates, providers should update automatically based on new pinned versions, but with separate charts, this may require manual intervention or additional steps.

## Decision

- Use Go build tags to pin CAPI providers for community and Prime builds.
- If the Go build tag approach for pinning providers doesn't work well, switch to pinning with Helm charts. Since the provider chart is already merged, it won't be hard to add this functionality.
- To improve the air-gapped experience with Helm chart changes in Turtles, we will add a ConfigMap with bundled core CAPI components to the Turtles chart. This would make Rancher air-gapped installs easier by removing the need for any OCI usage for core CAPI, as all manifests would be bundled with Rancher through Turtles. Users would still need to handle image mirroring, as they do now.
- Evaluate the future of ClusterctlConfig CRD: It might become redundant if users can specify an OCI registry. However, `clusterctl.yaml` is not just for pinning versions, it also allows users to supply custom GitHub repositories for providers not hardcoded in the clusterctl library. This opens up better UX for certifying and supporting custom providers. We need to find out how much the ClusterctlConfig CRD is actually used, if there's not enough interest from users, we can start discussing its deprecation.

## Open questions

- `ClusterctlConfig` is a singleton resource, how to handle in a Helm chart, if a ClusterctlConfig already exists, should the chart fail, adopt it, or skip? Should we allow users to manage it themselves?

