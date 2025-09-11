#!/bin/sh
#
# Bumps Turtles version in a locally checked out rancher/charts repository
#
# Usage:
#   ./release-against-charts.sh <path to charts repo> <prev turtles release> <new turtles release> [bump major]
#
# Example:
# ./release-against-charts.sh "${GITHUB_WORKSPACE}" "v0.23.0-rc.0" "v0.23.0" false

CHARTS_DIR=$1
PREV_TURTLES_VERSION="$2"   # e.g. 0.23.0-rc.0
NEW_TURTLES_VERSION="$3"    # e.g. 0.23.0
BUMP_MAJOR="${4:-false}"   # default false if not given

usage() {
    cat <<EOF
Usage:
  $0 <path to charts repo> <prev turtles release> <new turtles release> [bump_major]

Arguments:
  <path to charts repo>   Path to locally checked out charts repo
  <prev turtles release>  Previous rancher-turtles version (e.g. v0.20.0-rc.0)
  <new turtles release>   New rancher-turtles version (e.g. v0.20.0, v0.20.1-rc.1, v0.21.0-rc.0)
  <bump_major>            Optional. Must be "true" if introducing a new turtles minor version.
                          Example: v0.20.0 → v0.21.0-rc.0 requires bump_major=true.

Examples:
  RC to RC:        $0 ./charts v0.19.0-rc.0 v0.19.0-rc.1
  RC to stable:    $0 ./charts v0.20.0-rc.0 v0.20.0
  stable to RC:    $0 ./charts v0.20.0 v0.20.1-rc.1
  new minor RC:    $0 ./charts v0.20.0 v0.21.0-rc.0 true   # bump chart major
EOF
}

# Bumps the patch version of a semver version string
# e.g. 1.2.3 -> 1.2.4
# e.g. 1.2.3-rc.4 -> 1.2.4
bump_patch() {
    version=$1
    major=$(echo "$version" | cut -d. -f1)
    minor=$(echo "$version" | cut -d. -f2)
    patch=$(echo "$version" | cut -d. -f3)
    new_patch=$((patch + 1))
    echo "${major}.${minor}.${new_patch}"
}

# Bumps the major version of a semver version string and resets minor and patch to 0
# e.g. 1.2.3 -> 2.0.0
# e.g. 1.2.3-rc.4 -> 2.0.0
bump_major() {
    version=$1
    major=$(echo "$version" | cut -d. -f1)
    new_major=$((major + 1))
    echo "${new_major}.0.0"
}

# Validates that the version is in the format v<major>.<minor>.<patch> or v<major>.<minor>.<patch>-rc.<number>
validate_version_format() {
    version=$1
    if ! echo "$version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$'; then
        echo "Error: Version $version must be in the format v<major>.<minor>.<patch> or v<major>.<minor>.<patch>-rc.<number>"
        exit 1
    fi
}

if [ -z "$CHARTS_DIR" ] || [ -z "$PREV_TURTLES_VERSION" ] || [ -z "$NEW_TURTLES_VERSION" ]; then
    usage
    exit 1
fi

validate_version_format "$PREV_TURTLES_VERSION"
validate_version_format "$NEW_TURTLES_VERSION"

if echo "$PREV_TURTLES_VERSION" | grep -q '\-rc'; then
    is_prev_rc=true
else
    is_prev_rc=false
fi

if [ "$PREV_TURTLES_VERSION" = "$NEW_TURTLES_VERSION" ]; then
    echo "Previous and new rancher turtles version are the same: $NEW_TURTLES_VERSION, but must be different"
    exit 1
fi

# Remove the prefix v because the chart version doesn't contain it
PREV_TURTLES_VERSION_SHORT=$(echo "$PREV_TURTLES_VERSION" | sed 's|^v||')  # e.g. 0.23.0-rc.0
NEW_TURTLES_VERSION_SHORT=$(echo "$NEW_TURTLES_VERSION" | sed 's|^v||')  # e.g. 0.23.0

# Extract base versions without -rc suffix
prev_base=$(echo "$PREV_TURTLES_VERSION_SHORT" | sed 's/-rc.*//')
new_base=$(echo "$NEW_TURTLES_VERSION_SHORT" | sed 's/-rc.*//')

prev_minor=$(echo "$prev_base" | cut -d. -f2)
new_minor=$(echo "$new_base" | cut -d. -f2)

is_new_minor=false
if [ "$new_minor" -gt "$prev_minor" ]; then
    is_new_minor=true
fi

set -ue

cd "${CHARTS_DIR}"

# Validate the given turtles version (eg: 0.23.0-rc.0)
if ! grep -q "${PREV_TURTLES_VERSION_SHORT}" ./packages/rancher-turtles/package.yaml; then
    echo "Previous turtles version references do not exist in ./packages/rancher-turtles/. The content of the file is:"
    cat ./packages/rancher-turtles/package.yaml
    exit 1
fi

# Get the chart version (eg: 107.0.0)
if ! PREV_CHART_VERSION=$(yq '.version' ./packages/rancher-turtles/package.yaml); then
    echo "Unable to get chart version from ./packages/rancher-turtles/package.yaml. The content of the file is:"
    cat ./packages/rancher-turtles/package.yaml
    exit 1
fi

# Determine new chart version
if [ "$is_new_minor" = "true" ]; then
    if [ "$BUMP_MAJOR" != "true" ]; then
        echo "Error: Detected new minor bump ($PREV_TURTLES_VERSION to $NEW_TURTLES_VERSION), but bump_major flag was not set."
        exit 1
    fi
    echo "Bumping chart major: $PREV_CHART_VERSION to $(bump_major "$PREV_CHART_VERSION")"
    NEW_CHART_VERSION=$(bump_major "$PREV_CHART_VERSION")
    COMMIT_MSG="Bump rancher-turtles to $NEW_TURTLES_VERSION (chart version major bump)"
elif [ "$is_prev_rc" = "false" ]; then
    echo "Bumping chart patch: $PREV_CHART_VERSION to $(bump_patch "$PREV_CHART_VERSION")"
    NEW_CHART_VERSION=$(bump_patch "$PREV_CHART_VERSION")
    COMMIT_MSG="Bump rancher-turtles to $NEW_TURTLES_VERSION (chart version patch bump)"
else
    echo "Keeping chart version unchanged: $PREV_CHART_VERSION"
    NEW_CHART_VERSION=$PREV_CHART_VERSION
    COMMIT_MSG="Bump rancher-turtles to $NEW_TURTLES_VERSION (no chart version bump)"
fi

sed -i "s/${PREV_TURTLES_VERSION_SHORT}/${NEW_TURTLES_VERSION_SHORT}/g" ./packages/rancher-turtles/package.yaml
sed -i "s/${PREV_CHART_VERSION}/${NEW_CHART_VERSION}/g" ./packages/rancher-turtles/package.yaml

git add packages/rancher-turtles
git commit -m "$COMMIT_MSG"

PACKAGE=rancher-turtles make charts
git add ./assets/rancher-turtles ./charts/rancher-turtles index.yaml
git commit -m "make charts"

# Prepends to list
yq --inplace ".rancher-turtles = [\"${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT}\"] + .rancher-turtles" release.yaml

git add release.yaml
git commit -m "Add rancher-turtles ${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT} to release.yaml"