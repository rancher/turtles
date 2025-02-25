<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [14. Turtles UI installation](#title)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Turtles UI installation

- Status: proposed
- Date: 2025-02-17
- Authors: @Danil-Grigorev
- Deciders: @alexander-demicev @furkatgofurov7 @salasberryfin @anmazzotti @mjura @yiannistri

## Context

Turtles UI [extension][] provides UI functionality for the Turtles backend. Current installation procedure for Rancher involves set of mandatory steps described in documentation, which involves:
- [Installing rancher turtles][turtles-install] chart via dashboard
- [Installing UI][ui-install] extension via dashboard

This process is more complicated then a combined and automated installation, and may also lead to issues like:
- Missed UI extension installation step
- Installation of incompatible version of UI extension and Rancher Turles (involving CAPI version)
- Invalid combination of Turtles and UI versions in case of Turtles chart upgrade

[extension]: https://github.com/rancher/capi-ui-extension
[turtles-install]: https://turtles.docs.rancher.com/turtles/stable/en/getting-started/install-rancher-turtles/using_rancher_dashboard.html#_installation
[ui-install]: https://turtles.docs.rancher.com/turtles/stable/en/getting-started/install-rancher-turtles/using_rancher_dashboard.html#_capi_ui_extension_installation

## Decision

The proposed solution is to install UI extension chart as a `Helm` dependency for the Turtles `Helm` chart. This will leverage `questions.yaml` [integration][] to allow users to configure extension settings or disable UI chart installation.

UI extensions use `cattle-ui-plugin-system` namespace. Turtles `helm` chart will create CAPI UI `UIPlugin` resource, if the `turtlesUI.enabled` helm value is set.

Turtles chart will manage the lifecycle of UI extension by setting ownership references on the `UIPlugin` resource, to ensure automatic deletion on chart removal and moving the resource to the `cattle-ui-plugin-system` namespace.

Feature will be opt-in only to support alternative installation paths in the future.

[integration]: https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/helm-charts-in-rancher/create-apps#questionsyml

## Consequences

- UI extension will be managed by Turtles chart
- Existing UI extension installation will be adopted by Turtles chart upgrade
- UI extension version will be seamlessly updated with Turtles chart upgrade
