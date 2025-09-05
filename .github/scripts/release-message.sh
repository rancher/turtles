#!/bin/sh
#
# Automatically generates a message for a new release of turtles with some useful
# links and embedded release notes.
#
# Usage:
#   ./release-message.sh <prev turtles release> <new turtles release>
#
# Example:
# ./release-message.sh "v0.23.0-rc.0" "v0.23.0-rc.1"

PREV_TURTLES_VERSION=$1   # e.g. v0.23.0-rc.0
NEW_TURTLES_VERSION=$2    # e.g. v0.23.0-rc.1
GITHUB_TRIGGERING_ACTOR=${GITHUB_TRIGGERING_ACTOR:-}

usage() {
    cat <<EOF
Usage:
  $0 <prev turtles release> <new turtles release>
EOF
}

if [ -z "$PREV_TURTLES_VERSION" ] || [ -z "$NEW_TURTLES_VERSION" ]; then
    usage
    exit 1
fi

set -ue

url=$(gh release view --repo rancher/turtles --json url "${NEW_TURTLES_VERSION}" --jq '.url')
body=$(gh release view --repo rancher/turtles --json body "${NEW_TURTLES_VERSION}" --jq '.body')

generated_by=""
if [ -n "$GITHUB_TRIGGERING_ACTOR" ]; then
    generated_by=$(cat <<EOF
# About this PR

The workflow was triggered by $GITHUB_TRIGGERING_ACTOR.
EOF
)
fi

cat <<EOF
# Release note for [${NEW_TURTLES_VERSION}]($url)

$body

# Useful links

- Commit comparison: https://github.com/rancher/turtles/compare/${PREV_TURTLES_VERSION}...${NEW_TURTLES_VERSION}
- Release ${PREV_TURTLES_VERSION}: https://github.com/rancher/turtles/releases/tag/${PREV_TURTLES_VERSION}

$generated_by
EOF