# Rancher Turtles Release

This document describes the new release process for Turtles.

## Release Cadence

- New Rancher Turtles versions are usually released every month. Typically this happens a couple of weeks before the Rancher release that includes the Turtles release.

## Release Process

We maintain 3 release branches for Turtles, one for each of the minor versions of Rancher that is under active maintenance, at any given point. This allows us to create new Turtles releases for each of the release branches as required for bug fixes and security patches. The process of cutting a new release is essentially the following:
- [Create a new tag for the release](#create-a-new-tag-for-the-release)
- [Update `rancher/charts` repository](#update-ranchercharts-repository)
- [Update `rancher/rancher` repository](#update-rancherrancher-repository)

### Create a new tag for the release

**Note**: The commands shown in this section assume that you are working on your personal fork of `rancher/turtles`. Your set of repositories (`remotes`) should include a repository named `upstream` which is used for tracking branches in the `rancher/turtles` repository.

Creating a new tag on `rancher/turtles` triggers a GitHub Actions [workflow](https://github.com/rancher/turtles/actions/workflows/release-v2.yaml) that builds the container images for the given tag and pushes them to their respective registries.

To create a new RC (for example `v0.25.1-rc.1`) from release branch `release/v0.25` you need to follow these steps:

1. Update your local release branch

```bash
export RELEASE_BRANCH='release/v0.25'
git fetch upstream
git switch -c ${RELEASE_BRANCH} || git switch ${RELEASE_BRANCH}
git pull
```

2. Create a new tag for the new RC

```bash
export RELEASE_TAG='v0.25.1-rc.1'
git tag -s -a ${RELEASE_TAG} -m ${RELEASE_TAG}
git tag -s -a test/${RELEASE_TAG} -m "Testing framework ${RELEASE_TAG}"
git tag -s -a examples/${RELEASE_TAG} -m "ClusterClass examples ${RELEASE_TAG}"
git push upstream ${RELEASE_TAG}
git push upstream test/${RELEASE_TAG}
git push upstream examples/${RELEASE_TAG}
```

### Update `rancher/charts` repository

**Warning**: Before updating `rancher/charts` repository, ensure that the previous [step](#create-a-new-tag-for-the-release) has created a new GitHub release for the tag. This release must include the Helm chart archive file in its assets (typically named `rancher-turtles-${RELEASE_TAG}.tgz`).

This part of the release is automated via a GitHub Actions workflow that needs to be invoked manually from GitHub. To invoke it, navigate to the [workflow](https://github.com/rancher/turtles/actions/workflows/release-against-charts.yml), select the option `Run workflow` from the UI and pass the following parameters:
- Use workflow from: Branch: main
  This parameter should be set to the release branch that was used for creating the tag, for example `release/v0.25`. Using `main` branch may also work but using the release branch is safer, in case there are differences in the release workflows between these branches.
- Submit PR against the following rancher/charts branch (e.g. dev-v2.12): dev-v2.12
  This must be set to the `rancher/charts` branch that needs to be updated, with the new Turtles release. `dev-v2.12` is used for Rancher 2.12.x, `dev-v2.13` is used for Rancher 2.13.x and so on.
- Previous Turtles version (e.g. v0.23.0-rc.0)
  This is self explanatory, the value must be set to the previous Turtles version, for example `v0.25.1-rc.0`.
- New Turtles version (e.g. v0.23.0)
  This is self explanatory, the value must be set to the new Turtles version, for example `v0.25.1-rc.1`
- Set 'true' to bump chart major version when the Turtles minor version increases (e.g., v0.20.0 -> v0.21.0-rc.0). Default: false
  This is self explanatory, the values should be set to `true` when bumping the Turtles minor version, otherwise it should be set to `false`. 

Once this workflow has finished, a new PR should have been created in the `rancher/charts` repository that updates the selected branch with the new Turtles version. Here's an example (PR)[https://github.com/rancher/charts/pull/6294] from a previous run against the `dev-v2.13` branch. You need to review and merge this PR before proceeding to the next step.

### Update `rancher/rancher` repository

**Warning**: Before updating `rancher/rancher` repository, ensure that the PR generated from the previous [step](#update-ranchercharts-repository) has been merged.

This part of the release is also automated via a GitHub Actions workflow, that needs to be invoked manually from GitHub. To invoke it, navigate to the [workflow](https://github.com/rancher/turtles/actions/workflows/release-against-rancher.yml), select the option `Run workflow` from the UI and pass the following parameters:
- Use workflow from: Branch: main
  This parameter should be set to the release branch that was used for creating the tag, for example `release/v0.25`. Using `main` branch may also work but using the release branch is safer, in case there are differences in the release workflows between these branches.
- Submit PR against the following rancher/rancher branch (e.g. release/v2.12): release/v2.12
  This must be set to the `rancher/rancher` branch that needs to be updated, with the new Turtles release. `release/v2.12` is used for Rancher 2.12.x, `release/v2.13` (which is not yet present) will be used for Rancher 2.13.x and so on.
- Previous Turtles version (e.g. v0.23.0-rc.0)
  This is self explanatory, the value must be set to the previous Turtles version, for example `v0.25.1-rc.0`.
- New Turtles version (e.g. v0.23.0)
  This is self explanatory, the value must be set to the new Turtles version, for example `v0.25.1-rc.1`
- Set 'true' to bump chart major version when the Turtles minor version increases (e.g., v0.23.0 -> v0.24.0-rc.0). Default: false
  This is self explanatory, the values should be set to `true` when bumping the Turtles minor version, otherwise it should be set to `false`. 

Once this workflow has finished, a new PR should have been created in the `rancher/rancher` repository that updates the selected branch with the new Turtles version. You need to review and merge this PR. When this PR gets merged, you will have completed the process of releasing a new version of Turtles and including it in an upcoming version of Rancher.

## Versioning

Rancher Turtles follows [semantic versioning](https://semver.org/) specification.

Example versions:
- Pre-release: `v0.4.0-alpha.1`
- Minor release: `v0.4.0`
- Patch release: `v0.4.1`
- Major release: `v1.0.0`

With the v0 release of our codebase, we provide the following guarantees:

- A (*minor*) release CAN include:
  - Introduction of new API versions, or new Kinds.
  - Compatible API changes like field additions, deprecation notices, etc.
  - Breaking API changes for deprecated APIs, fields, or code.
  - Features, promotion or removal of feature gates.
  - And more!

- A (*patch*) release SHOULD only include backwards compatible set of bugfixes.

### Backporting

Any backport MUST not be breaking for either API or behavioral changes.

It is generally not accepted to submit pull requests directly against release branches (release/X). However, backports of fixes or changes that have already been merged into the main branch may be accepted to all supported branches:

- Critical bugs fixes, security issue fixes, or fixes for bugs without easy workarounds.
- Dependency bumps for CVE (usually limited to CVE resolution; backports of non-CVE related version bumps are considered exceptions to be evaluated case by case)
- Cert-manager version bumps (to avoid having releases with cert-manager versions that are out of support, when possible)
- Changes required to support new Kubernetes versions, when possible. See supported Kubernetes versions for more details.
- Changes to use the latest Go patch version to build controller images.
- Improvements to existing docs (the latest supported branch hosts the current version of the book)

**Note:** We generally do not accept backports to Rancher Turtles release branches that are out of support.

#### Automation to create backports

There is automation in place to create backport PRs when a comment of a certain format is added to the original PR:

```
/backport <milestone> <target_branch>
```

For example, adding the comment `/backport v2.12.1 release/v0.25` to a PR will result in creating a new backport PR against release branch `release/v0.25` with a patch from the PR that the comment was added to and a milestone set to `v2.12.1`. [Here](https://github.com/rancher/turtles/pull/1782) is an example of a backport PR generated through this automation.

## Branches

Rancher Turtles has two types of branches: the `main` and `release/X` branches. Before integrating with Rancher release branches were named `release-X` but since then `release/X` is used.

The `main` branch is where development happens. All the latest and greatest code, including breaking changes, happens on main.

The `release/X` branches contain stable, backwards compatible code. On every major or minor release, a new branch is created. It is from these branches that minor and patch releases are tagged. In some cases, it may be necessary to open PRs for bugfixes directly against stable branches, but this should generally not be the case.

### Support and guarantees

Rancher Turtles maintains the most recent release/releases for all supported APIs. Support for this section refers to the ability to backport and release patch versions; [backport policy](#backporting) is defined above.

- The API version is determined from the GroupVersion defined in the top-level `api/` package.
- For the current stable API version (v1alpha1) we support the two most recent minor releases; older minor releases are immediately unsupported when a new major/minor release is available.
