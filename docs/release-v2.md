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

To create a new RC (for example `0.25.1-rc.1`) from release branch `release/v0.25` you need to follow these steps:

1. Update your local release branch

```bash
export RELEASE_BRANCH='release/v0.25'
git fetch upstream
git switch -c ${RELEASE_BRANCH} || git switch ${RELEASE_BRANCH}
git pull
```

2. Create a new tag for the new RC

```bash
export RELEASE_TAG='0.25.1-rc.1'
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
  This is self explanatory, the value must be set to the previous Turtles version, for example `0.25.1-rc.0`.
- New Turtles version (e.g. v0.23.0)
  This is self explanatory, the value must be set to the new Turtles version, for example `0.25.1-rc.1`
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
  This is self explanatory, the value must be set to the previous Turtles version, for example `0.25.1-rc.0`.
- New Turtles version (e.g. v0.23.0)
  This is self explanatory, the value must be set to the new Turtles version, for example `0.25.1-rc.1`
- Set 'true' to bump chart major version when the Turtles minor version increases (e.g., v0.23.0 -> v0.24.0-rc.0). Default: false
  This is self explanatory, the values should be set to `true` when bumping the Turtles minor version, otherwise it should be set to `false`. 

Once this workflow has finished, a new PR should have been created in the `rancher/rancher` repository that updates the selected branch with the new Turtles version. You need to review and merge this PR. When this PR gets merged, you will have completed the process of releasing a new version of Turtles and including it in an upcoming version of Rancher.