#!/bin/sh
#
# Bumps Turtles version in a locally checked out rancher/rancher repository
#
# Usage:
#   ./release-against-rancher.sh <path to rancher repo> <new turtles release>
#
# Example:
# ./release-against-rancher.sh "${GITHUB_WORKSPACE}" "v0.23.0"

RANCHER_DIR=$1
NEW_TURTLES_VERSION=$2   # e.g. v0.23.0-rc.0

usage() {
    cat <<EOF
Usage:
  $0 <path to rancher repo> <new turtles release>
EOF
}

# Bumps the patch version of a semver version string
# e.g. 1.2.3 -> 1.2.4
# e.g. 1.2.3-rc.4 -> 1.2
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

if [ -z "$RANCHER_DIR" ] || [ -z "$NEW_TURTLES_VERSION" ]; then
    usage
    exit 1
fi

validate_version_format "$NEW_TURTLES_VERSION"

# Remove the prefix v because the chart version doesn't contain it
NEW_TURTLES_VERSION_SHORT=$(echo "$NEW_TURTLES_VERSION" | sed 's|^v||')  # e.g. 0.5.2-rc.3

set -ue

pushd "${RANCHER_DIR}" > /dev/null

# Get the turtles version (eg: 0.5.0-rc.12)
if ! PREV_TURTLES_VERSION_SHORT=$(yq -r '.turtlesVersion' ./build.yaml | sed 's|.*+up||'); then
    echo "Unable to get turtles version from ./build.yaml. The content of the file is:"
    cat ./build.yaml
    exit 1
fi

if [ "$PREV_TURTLES_VERSION_SHORT" = "$NEW_TURTLES_VERSION_SHORT" ]; then
    echo "Previous and new turtles version are the same: $NEW_TURTLES_VERSION, but must be different"
    exit 1
fi

if echo "$PREV_TURTLES_VERSION_SHORT" | grep -q '\-rc'; then
    is_prev_rc=true
else
    is_prev_rc=false
fi

# Get the chart version (eg: 104.0.0)
if ! PREV_CHART_VERSION=$(yq -r '.turtlesVersion' ./build.yaml | cut -d+ -f1); then
    echo "Unable to get chart version from ./build.yaml. The content of the file is:"
    cat ./build.yaml
    exit 1
fi

if [ "$is_prev_rc" = "false" ]; then
    NEW_CHART_VERSION=$(bump_patch "$PREV_CHART_VERSION")
else
    NEW_CHART_VERSION=$PREV_CHART_VERSION
fi

yq --inplace ".turtlesVersion = \"${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT}\"" ./build.yaml

# Downloads dapper
make .dapper

# DAPPER_MODE=bind will make sure we output everything that changed
DAPPER_MODE=bind ./.dapper go generate ./... || true
DAPPER_MODE=bind ./.dapper rm -rf go .config

git add .
git commit -m "Bump turtles to ${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION_SHORT}"

popd > /dev/null