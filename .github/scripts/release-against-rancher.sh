#!/bin/bash
#
# Sets turtlesVersion in build.yaml of a rancher/rancher checkout to the chart of a new Turtles
# release, regenerates the files that depend on it and commits the result.
#
# Usage:
#   release-against-rancher.sh <rancher repo> <new turtles version> <chart version>
#
# Example:
#   release-against-rancher.sh ./rancher v0.28.0 111.0.0+up0.28.0
#
# The chart version is the one published in rancher/charts: it is not worked out here.

RANCHER_DIR=$1
NEW_TURTLES_VERSION=$2   # e.g. v0.28.0 or v0.28.0-rc.7
NEW_CHART_VERSION=$3     # e.g. 111.0.0+up0.28.0

usage() {
    cat <<EOF
Usage:
  $0 <rancher repo> <new turtles version> <chart version>

Arguments:
  <rancher repo>          Path to a local checkout of rancher/rancher
  <new turtles version>   New Turtles version, e.g. v0.28.0 or v0.28.0-rc.7
  <chart version>         Version of the rancher-turtles chart in rancher/charts, e.g. 111.0.0+up0.28.0
EOF
}

if [ -z "$RANCHER_DIR" ] || [ -z "$NEW_TURTLES_VERSION" ] || [ -z "$NEW_CHART_VERSION" ]; then
    usage
    exit 1
fi

# The Turtles version must be vX.Y.Z or vX.Y.Z-rc.N.
if ! echo "$NEW_TURTLES_VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$'; then
    echo "Error: Version $NEW_TURTLES_VERSION must be in the format v<major>.<minor>.<patch> or v<major>.<minor>.<patch>-rc.<number>"
    exit 1
fi

# The chart version must be for this Turtles version: 111.0.0+up0.28.0 is for v0.28.0.
if [ "${NEW_CHART_VERSION##*+up}" != "${NEW_TURTLES_VERSION#v}" ]; then
    echo "Error: chart version $NEW_CHART_VERSION is not for Turtles $NEW_TURTLES_VERSION"
    exit 1
fi

set -ue

cd "$RANCHER_DIR"

if ! PREV_CHART_VERSION=$(yq -r '.turtlesVersion' ./build.yaml); then
    echo "Unable to get turtles version from ./build.yaml. The content of the file is:"
    cat ./build.yaml
    exit 1
fi

if [ "$PREV_CHART_VERSION" = "$NEW_CHART_VERSION" ]; then
    echo "Previous and new turtles version are the same: $NEW_CHART_VERSION, but must be different"
    exit 1
fi

yq --inplace ".turtlesVersion = \"${NEW_CHART_VERSION}\"" ./build.yaml
go generate ./...

git add .
git commit -m "Bump turtles to ${NEW_CHART_VERSION}"
