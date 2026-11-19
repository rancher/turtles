#!/usr/bin/env bash
# Prepares the chart of a release for the ship-staged action of rancher/charts. It builds the
# chart the way a release does, with `make build-chart`, in a checkout of the tag, and copies
# it to <output dir>/rancher-turtles with the appVersion of the release set, because the
# chart in the repository has a placeholder there. The action stamps, checks, packages and
# pushes the copy, so the checkout stays as it is.
#
# The annotations of the chart, among them catalog.cattle.io/rancher-version and
# catalog.cattle.io/kube-version, are not changed here: they say which Rancher and Kubernetes
# versions a release supports, and the catalog needs both bounds in each.
#
# Usage: TAG=<tag> prepare-charts.sh <release checkout> <output dir>
#   e.g. TAG=v0.28.0 prepare-charts.sh release "$RUNNER_TEMP/charts"
#
# Requires what `make build-chart` needs: Go, and the tools it downloads. Also yq.
set -euo pipefail

REL=${1:?usage: TAG=<tag> $0 <release checkout> <output dir>}
OUT=${2:?usage: TAG=<tag> $0 <release checkout> <output dir>}
TAG=${TAG:?TAG is required, such as v0.28.0}

if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "Error: tag $TAG must look like v0.28.0 or v0.28.0-rc.5" >&2
  exit 1
fi
if [ ! -f "$REL/Makefile" ] || [ ! -d "$REL/charts/rancher-turtles" ]; then
  echo "Error: $REL is not a checkout of rancher/turtles" >&2
  exit 1
fi

# The same variables the release workflow gives to make: the tag, and the image of the
# controller, rancher/turtles.
(cd "$REL" && RELEASE_TAG="$TAG" CONTROLLER_IMG=rancher/turtles ORG=rancher make build-chart)

rm -rf "$OUT/rancher-turtles"
mkdir -p "$OUT"
cp -r "$REL/out/charts/rancher-turtles" "$OUT/rancher-turtles"
APP_VERSION="${TAG#v}" yq -i '.appVersion = strenv(APP_VERSION)' "$OUT/rancher-turtles/Chart.yaml"
