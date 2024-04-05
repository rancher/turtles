# Rancher Turtles Releases

## Release Cadence

- New Rancher Turtles versions are usually released every 2-4 weeks.

## Release Process

1. Clone the repository locally: 

```bash
git clone git@github.com:rancher/turtles.git
```

2. Depending on whether you are cutting a minor/major or patch release, the process varies.

    * If you are cutting a new minor/major release:

        Create a new release branch (i.e release-X) and push it to the upstream repository.

        ```bash
            # Note: `upstream` must be the remote pointing to `github.com/rancher/turtles`.
            git checkout -b release-0.4
            git push -u upstream release-0.4
            # Export the tag of the minor/major release to be cut, e.g.:
            export RELEASE_TAG=v0.4.0
        ```
    * If you are cutting a patch release from an existing release branch:

        Use existing release branch.

        ```bash
            # Note: `upstream` must be the remote pointing to `github.com/rancher/turtles`
            git checkout upstream/release-0.4
            # Export the tag of the patch release to be cut, e.g.:
            export RELEASE_TAG=v0.4.1
        ```
3. Create a signed/annotated tag and push it:

```bash
# Create tags locally
git tag -s -a ${RELEASE_TAG} -m ${RELEASE_TAG}

# Push tags
git push upstream ${RELEASE_TAG}
```

This will trigger a [release GitHub action](https://github.com/rancher/turtles/actions/workflows/release.yaml) that creates a release with Rancher Turtles components.

**Note:** If you are cutting a new minor/major release, please follow the [next step](#post-release-steps-in-rancher-turtles-docs) below, otherwise skip.

## Post-release steps in Rancher Turtles Docs

If a new minor/major branch was created, there are some post-release actions that need to be taken.

**Note:** Before moving one, please make sure to ask a team member or directly set branch *Enforcement status* to **Disabled** in the Rancher Turtles Docs repository by navigating [here](https://github.com/rancher/turtles-docs/settings/rules/124836).

### Create a new tag for Rancher Turtles Docs repo

1. Clone the Rancher Turtles Docs repository locally: 

```bash
git clone git@github.com:rancher/turtles-docs.git
```

2. Export the tag of the minor/major release, create a signed/annotated tag and push it:

```bash
# Export the tag of the minor/major release in a format of v0.X/v1.X, e.g.:
export RELEASE_TAG=v0.4

# Create tags locally
git tag -s -a ${RELEASE_TAG} -m ${RELEASE_TAG}

# Push tags
# Note: `upstream` must be the remote pointing to `github.com/rancher/turtles-docs`
git push upstream ${RELEASE_TAG}
```

### Automatic patch creation for new version of Rancher Turtles Docs

This step is automatic, however it requires temporarily adjusting repo permissions so that the new docs version patch is pushed on behalf of the tag committer (example new docs version [patch](https://github.com/rancher/turtles-docs/commit/e7e37a92b6e48c6f4bc0de386a0003bb2b9d610b)).

1. Wait for the [version publish workflow](https://github.com/rancher/turtles-docs/actions/workflows/version-publish.yaml) to finish.

2. Ask a team member or directly set branch *Enforcement status* back to **Active** in the Rancher Turtles Docs repository by navigating [here](https://github.com/rancher/turtles-docs/settings/rules/124836).

### Deploy site to Pages

Next, we need to deploy site to Github Pages:

1. Manually run [publish workflow](https://github.com/rancher/turtles-docs/actions/workflows/publish.yaml) to force rollout of updated docs version.

Once all steps above are completed, a new version of Rancher Turtles Docs should be available at [https://turtles.docs.rancher.com].

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

It is generally not accepted to submit pull requests directly against release branches (release-X). However, backports of fixes or changes that have already been merged into the main branch may be accepted to all supported branches:

- Critical bugs fixes, security issue fixes, or fixes for bugs without easy workarounds.
- Dependency bumps for CVE (usually limited to CVE resolution; backports of non-CVE related version bumps are considered exceptions to be evaluated case by case)
- Cert-manager version bumps (to avoid having releases with cert-manager versions that are out of support, when possible)
- Changes required to support new Kubernetes versions, when possible. See supported Kubernetes versions for more details.
- Changes to use the latest Go patch version to build controller images.
- Improvements to existing docs (the latest supported branch hosts the current version of the book)

**Note:** We generally do not accept backports to Rancher Turtles release branches that are out of support.

## Branches

Rancher Turtles has two types of branches: the `main` and `release-X` branches.

The `main` branch is where development happens. All the latest and greatest code, including breaking changes, happens on main.

The `release-X` branches contain stable, backwards compatible code. On every major or minor release, a new branch is created. It is from these branches that minor and patch releases are tagged. In some cases, it may be necessary to open PRs for bugfixes directly against stable branches, but this should generally not be the case.

### Support and guarantees

Rancher Turtles maintains the most recent release/releases for all supported APIs. Support for this section refers to the ability to backport and release patch versions; [backport policy](#backporting) is defined above.

- The API version is determined from the GroupVersion defined in the top-level `api/` package.
- For the current stable API version (v1alpha1) we support the two most recent minor releases; older minor releases are immediately unsupported when a new major/minor release is available.