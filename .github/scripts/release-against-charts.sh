#!/bin/bash
#
# Submit new Turtles version against rancher/charts

set -e

PREV_TURTLES_VERSION="$1"   # e.g. 0.23.0-rc.0
NEW_TURTLES_VERSION="$2"
PREV_CHART_VERSION="$3"   # e.g. 101.2.0
NEW_CHART_VERSION="$4"
REPLACE="$5"              # remove previous version if `true`, otherwise add new

CHARTS_DIR=${CHARTS_DIR-"$(dirname -- "$0")/../../../charts"}

pushd "${CHARTS_DIR}" > /dev/null

if [ ! -e ~/.gitconfig ]; then
    git config user.name "$GITHUB_ACTOR"
    git config user.email "$GITHUB_ACTOR@users.noreply.github.com"
fi

if [ -f packages/turtles/package.yaml ];  then
    # Use new auto bump scripting until the Github action CI works as expected
    # no parameters besides the target branch are needed in theory, but the pr
    # creation still needs the new Chart and Turtles version
    make chart-bump package=turtles branch="$(git rev-parse --abbrev-ref HEAD)"

    if [ "${REPLACE}" == "true" ] && [ -f "assets/turtles/turtles-${PREV_CHART_VERSION}+up${PREV_TURTLES_VERSION}.tgz" ]; then
        for i in turtles turtles-crd; do
            CHART=$i VERSION=${PREV_CHART_VERSION}+up${PREV_TURTLES_VERSION} make remove
        done
        git add assets/turtles* charts/turtles* index.yaml
        git commit -m "Remove Turtles ${PREV_CHART_VERSION}+up${PREV_TURTLES_VERSION}"
        git checkout release.yaml   # reset unwanted changes to release.yaml, relevant ones are already part of the previous commit
    fi
fi

PACKAGE=turtles make validate

popd > /dev/null