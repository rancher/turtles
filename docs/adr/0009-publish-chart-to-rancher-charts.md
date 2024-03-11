<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [9. Helm chart repository](#9-helm-chart-repository)
  - [Context](#context)
  - [Proposed alternatives](#proposed-alternatives)
  - [Consequences](#consequences)
  - [Decision](#decision)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 9. Helm chart repository

* Status: proposed
* Date: 2024-03-11
* Authors: @salasberryfin
* Deciders: @richardcase @alexander-demicev @Danil-Grigorev @mjura @furkatgofurov7

## Context

As we progress towards Rancher Turtles becoming GA, we have to decide how the Helm chart is made available to users. There are two alternatives to serve the chart:
- Through the official `rancher/charts`.
- Creating a Rancher Turtles specific Helm chart repository.

There are examples of existing Rancher projects using one or the other, so we should analyze which of the two we choose.

The chart will be published for Rancher `v2.8` and `v2.9`.

## Proposed alternatives

Analyze pros/cons of each solution and make a decision based on team feedback.

Before diving into each option, applications are installable from the Rancher Marketplace if the chart is available via a repository added to Rancher. This means that using a custom Rancher Turtles chart repository would translate in having to add the repository (can be done via UI) before installing (similar to [Kubewarden](https://github.com/kubewarden)). In future iterations, there could be a new ADR on how to made the Turtles installation available via Rancher Extensions (you can refer to [here](https://ranchermanager.docs.rancher.com/integrations-in-rancher/rancher-extensions) to read how Kubewarden and other Rancher Extension can be installed via the UI).

### Option 1 - `rancher/charts`.

This means publishing the Helm chart to the rancher chart repository: [rancher/charts](https://github.com/rancher/charts). The current configuration, which serves Helm charts using our own `rancher-turtles` repository, will then be deprecated. If hosting the CAPI Extension in `rancher/charts`, it will become automatically available through Rancher Marketplace.

Based on the existing processes used for other projects (hosted providers, fleet, etc.), we'll configure a GitHub Action that's triggered manually each time the chart needs to be updated, which automatically creates a PR against the `rancher/charts` repository and publishes the built chart.

This approach can add complexity to the release process, as we're no longer in charge of making the charts available. This adds an extra dependency on a team outside of Rancher Turtles development.

We also must comply with a number of rules to publish charts to `rancher/charts`.

#### Chart versioning

The major version of the chart represents the Rancher version it corresponds to:
- `102`: Rancher 2.7
- `103`: Rancher 2.8
- etc.

#### Version annotations

There are two annotations: `catalog.cattle.io/rancher-version` and `catalog.cattle.io/kube-version` which must include lower and upper bounds. Since this may vary from one Rancher major version to another, the easiest approach would be to have this removed from the original chart and manually published to `rancher/charts` when a new Rancher major version is released (refer to [fleet chart](https://github.com/rancher/fleet/blob/master/charts/fleet/Chart.yaml)). Subsequent updates to the chart will just use the existing bounds.

#### Chart dependencies

Dependencies must be added to the Rancher file `pkg/image/origins.go` which denotes the source code repository of each image used in chart.

Rancher Turtles has Cluster API Operator as a dependency, which also adds the Cert Manager dependency. `cert-manager` defines a `kubeVersion` field in `Chart.yaml` which would force us to annotate the chart. This is probably a workaround we would like to avoid.

### Option 2 - specific rancher-turtles repository.

So far, we've used the `rancher-turtles` repository as a Helm repository using [helm/chart-releaser](https://github.com/helm/chart-releaser). We could continue using this mechanism for hosting the chart and, since Turtles is now part of the Rancher organization in GitHub, we have a production-ready repository URL at https://rancher.github.io/turtles.

This means that the release workflow manages the end-to-end release process, including publishing the Helm chart, which will automatically become available after the release is created. On the other hand, it separates the chart from Rancher even though Rancher Turtles can only be installed together with Rancher. This differs from other projects that use a similar approach for chart management, like Kubewarden that is technically independent of Rancher.

## Consequences

### Using `rancher-charts`
- The existing publish mechanism based on `chart-releaser` will be deprecated.
- Hard to achieve a high level of automation.
- Chart repository is accessible from Rancher Marketplace.
- Release process will now require an extra step to create the `rancher/charts` PR: documentation must be aligned with the new configuration.
- First chart release for each major version of Rancher will require a manual update to the charts to set the correct version bounds as annotations. Subsequent updates should be applied via GitHub Actions.
    - Fixing dependency charts.
- This change should be transparent to the user.

### Using specific repository
- The existing publish mechanism will remain.
- Already fully automated
- Chart repository must be added before installing from Rancher UI.
- There must be a procedure for air-gapped scenarios, which initially won't be supported.
- This change should be transparent to the user.

## Decision

After discussing both alternatives as a team, with input from product, the decision is to use a **custom Rancher Turtles Helm chart repository**. For now, we do not consider making the extension available via Rancher Marketplace and the chart can still be installed from Rancher UI after manually adding the repository, which now lives in https://rancher.github.io/turtles. In future iterations we could consider using the Kubewarden approach and get the UI extension to add this repository.

This decision allows us to continue using the same chart publishing process we've been using and will not require changes.

### As a user

This will have no consequences on user experience.
