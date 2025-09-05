#!/bin/bash
#
# Submit new Turtles version against rancher/rancher

set -ue

NEW_TURTLES_VERSION="$1"    # e.g. 0.23.0-rc.0
NEW_CHART_VERSION="$2"    # e.g. 101.1.0

RANCHER_DIR=${RANCHER_DIR-"$(dirname -- "$0")/../../../rancher"}

pushd "${RANCHER_DIR}" > /dev/null

# Check if version is available online
CHART_DEFAULT_BRANCH=$(grep "ARG CHART_DEFAULT_BRANCH=" package/Dockerfile | cut -d'=' -f2)
if ! curl -s --head --fail "https://github.com/rancher/charts/raw/${CHART_DEFAULT_BRANCH}/assets/turtles/turtles-${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION}.tgz" > /dev/null; then
    echo "Version ${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION} does not exist in the branch ${CHART_DEFAULT_BRANCH} in rancher/charts"
    exit 1
fi

if [ -e build.yaml ]; then
    sed -i -e "s/turtlesVersion: .*$/turtlesVersion: ${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION}/" build.yaml
    go generate
    git add build.yaml pkg/buildconfig/constants.go
else
    sed -i -e "s/ENV CATTLE_RANCHER_TURTLES_VERSION=.*$/ENV CATTLE_RANCHER_TURTLES_VERSION=${NEW_CHART_VERSION}+up${NEW_TURTLES_VERSION}/" package/Dockerfile
    git add package/Dockerfile
fi

git commit -m "Updating to Turtles v${NEW_TURTLES_VERSION}"

popd > /dev/null