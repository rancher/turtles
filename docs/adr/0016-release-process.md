<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [16. Release Process](#16-release-process)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 16. Release Process

- Status: proposed
- Date: 2025-09-28
- Authors: @yiannistri
- Deciders: @alexander-demicev @furkatgofurov7 @salasberryfin @anmazzotti @mjura

## Context

As part of making Turtles available as a system chart in Rancher, the release process needs to evolve accordingly. This evolution will be done in 2 phases:
- the intermediate phase, where work in underway to integrate it with Rancher.
- the final phase, where Turtles has been integrated and released with Rancher.

Currently, the release process is fairly simple:
- the release happens on a monthly cadence
- a single image is being produced, that can be used by both community and Prime users
- the image is being pushed to both ghcr.io as well as the Prime (production) registry

## Decision

For the intermediate phase:
- the existing release process will continue to be used on a monthly basis as is
- a new release process will be introduced which should allow differentiation of community vs Prime builds (via Go build tags)
- the new release process should build be able to build both types of images on demand but not push them to any Prime registry yet

For the final phase:
- Turtles is released with Rancher
- images for x.y.0 patch versions are pushed to DockerHub, then synced from there into Prime (production) registry
- images for x.y.1 patch versions onwards are pushed to the Prime (staging) registry, then synced from there into Prime (production) registry

## Consequences

TBD