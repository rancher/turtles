#!/usr/bin/env bash

# Copyright © 2023 - 2025 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xe

# Builds a local copy of the rancher/charts repository, for Rancher to use as its
# system charts repository in the dev environment and the e2e tests.
#
# Packages the chart into assets/rancher-turtles, and adds a reference to it in index.yaml.

RANCHER_CHARTS_REPO_DIR=${RANCHER_CHARTS_REPO_DIR}
RANCHER_CHART_DEV_VERSION=${RANCHER_CHART_DEV_VERSION}
RANCHER_CHARTS_BASE_BRANCH=${RANCHER_CHARTS_BASE_BRANCH}
CHART_RELEASE_DIR=${CHART_RELEASE_DIR}
HELM=${HELM}

rm -rf "$RANCHER_CHARTS_REPO_DIR"
git clone --depth 1 -b "$RANCHER_CHARTS_BASE_BRANCH" https://github.com/rancher/charts.git "$RANCHER_CHARTS_REPO_DIR"

"$HELM" package "$CHART_RELEASE_DIR" --app-version=dev --version="$RANCHER_CHART_DEV_VERSION" --destination="$RANCHER_CHARTS_REPO_DIR/assets/rancher-turtles"
entry=$(yq -o=json -I=0 '.version = "'"$RANCHER_CHART_DEV_VERSION"'" | .appVersion = "dev" | .urls = ["assets/rancher-turtles/rancher-turtles-'"$RANCHER_CHART_DEV_VERSION"'.tgz"]' "$CHART_RELEASE_DIR/Chart.yaml")
yq -i ".entries.rancher-turtles += [$entry]" "$RANCHER_CHARTS_REPO_DIR/index.yaml"

# Commit everything to a new repository with a single commit.
rm -rf "$RANCHER_CHARTS_REPO_DIR/.git"
git -C "$RANCHER_CHARTS_REPO_DIR" init --quiet --initial-branch="$RANCHER_CHARTS_BASE_BRANCH"
git -C "$RANCHER_CHARTS_REPO_DIR" config user.email "ci@rancher-turtles.local"
git -C "$RANCHER_CHARTS_REPO_DIR" config user.name "Rancher Turtles CI"
git -C "$RANCHER_CHARTS_REPO_DIR" add .
git -C "$RANCHER_CHARTS_REPO_DIR" commit --quiet -m "Added test chart $RANCHER_CHART_DEV_VERSION"
