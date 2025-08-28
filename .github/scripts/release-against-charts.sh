#!/bin/sh
#
# Bumps Turtles version in a locally checked out rancher/charts repository
#
# Usage:
#   ./release-against-charts.sh <path to charts repo> <prev turtles release> <new turtles release>
#
# Example:
# ./release-against-charts.sh "${GITHUB_WORKSPACE}" "v0.5.0-rc.13" "v0.5.0-rc.14"

CHARTS_DIR=$1
PREV_TURTLES_VERSION="$2"   # e.g. 0.23.0-rc.0
NEW_TURTLES_VERSION="$3"    # e.g. 0.23.0

usage() {
    cat <<EOF
Usage:
  $0 <path to charts repo> <prev rancher turtles release> <new rancher turtles release>
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
PREV_TURTLES_VERSION_SHORT=$(echo "$PREV_TURTLES_VERSION" | sed 's|^v||')  # e.g. 0.5.2-rc.3
NEW_TURTLES_VERSION_SHORT=$(echo "$NEW_TURTLES_VERSION" | sed 's|^v||')  # e.g. 0.5.2-rc.4

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

if [ "$is_prev_rc" = "false" ]; then
    NEW_CHART_VERSION=$(bump_patch "$PREV_CHART_VERSION")
else
    NEW_CHART_VERSION=$PREV_CHART_VERSION
fi

sed -i "s/${PREV_TURTLES_VERSION_SHORT}/${NEW_TURTLES_VERSION_SHORT}/g" ./packages/rancher-turtles/package.yaml
sed -i "s/${PREV_CHART_VERSION}/${NEW_CHART_VERSION}/g" ./packages/rancher-turtles/package.yaml

git add packages/rancher-turtles
git commit -m "Bump rancher-turtles to $NEW_TURTLES_VERSION"

PACKAGE=rancher-turtles make charts
git add ./assets/rancher-turtles ./charts/rancher-turtles index.yaml
git commit -m "make charts"

# Prepends to list
yq --inplace ".rancher-turtles = [\"${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT}\"] + .rancher-turtles" release.yaml

git add release.yaml
git commit -m "Add rancher-turtles ${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT} to release.yaml"