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

## Decision

TBD

## Open questions

- `ClusterctlConfig` is a singleton resource, how to handle in a Helm chart, if a ClusterctlConfig already exists, should the chart fail, adopt it, or skip? Should we allow users to manage it themselves?

