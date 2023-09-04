<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [1. Use ADRs to record decisions](#1-use-adrs-to-record-decisions)
  - [Context](#context)
  - [Decision](#decision)
  - [Consequences](#consequences)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# 1. Use ADRs to record decisions

* Status: proposed
* Date: 2023-07-11
* Authors: @richardcase
* Deciders: @richardcase @alexander-demicev @furkatgofurov7 @salasberryfin @Danil-Grigorev @mjura

## Context

Throughout the course of developing the project decisions will be made that are not covered specifically by RFCs (a.k.a proposals) or other design documentation. These decisions may come about via discussions in Slack, on issues/PRs or in meetings.

We need a lightweight way to record these decisions so they are easily discoverable by the contributors when they are looking to understand why something is done a particular way.

## Decision

The project will use [Architectural Decision Records (ADR)](https://adr.github.io/) to record decisions that are applicable across the project and that are not explicitly covered by a RFC/proposal/design doc.

Additionally, in the implementation of a RFC decisions may still be made via discussions and so ADRs should be created in this instance as well.

A [template](./0000-template.md) has been created based on prior work:

* <https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions>
* <https://adr.github.io/madr/>

## Consequences

When decisions are made that affect the entire project then these need to be recorded as an ADR.

For decisions that are made as part of discussion on issues or PRs a new label could be added `needs-adr` (or something similar) so that its explicit.

For decisions made on slack or in a meeting someone will need to take an action to create the PR with the ADR. Maintainers and contributors will need to decide when a "decision" has been made and ensure an ADR is created.
