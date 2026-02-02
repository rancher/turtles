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

RANCHER_CHARTS_REPO_DIR=${RANCHER_CHARTS_REPO_DIR}
RANCHER_CHART_DEV_VERSION=${RANCHER_CHART_DEV_VERSION}
RANCHER_CHARTS_BASE_BRANCH=${RANCHER_CHARTS_BASE_BRANCH}
CHART_RELEASE_DIR=${CHART_RELEASE_DIR}
HELM=${HELM}
# if CLEANUP is false:
# changes are applied on top of the existing local copy of the Rancher Charts repository.
CLEANUP=${CLEANUP:-true}

# Cleanup
if [ "$CLEANUP" = "true" ]; then
    rm -rf $RANCHER_CHARTS_REPO_DIR
    mkdir -p $RANCHER_CHARTS_REPO_DIR
    # Build and copy Turtles chart into Rancher Charts local repo
    git clone --depth 1 -b $RANCHER_CHARTS_BASE_BRANCH https://github.com/rancher/charts.git $RANCHER_CHARTS_REPO_DIR
    rm -rf $RANCHER_CHARTS_REPO_DIR/.git
    git -C $RANCHER_CHARTS_REPO_DIR init --initial-branch=$RANCHER_CHARTS_BASE_BRANCH
fi

mkdir -p $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION
cp -r $CHART_RELEASE_DIR/* $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION
# Populate Chart.yaml with correct version
yq -i '.version = "'$RANCHER_CHART_DEV_VERSION'"' $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION/Chart.yaml
yq -i '.appVersion = "dev"' $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION/Chart.yaml
yq -i '.urls[0] += "assets/rancher-turtles/rancher-turtles-'$RANCHER_CHART_DEV_VERSION'.tgz"' $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION/Chart.yaml
# Populate release.yaml and index.yaml
yq -i '.rancher-turtles += "'$RANCHER_CHART_DEV_VERSION'"' $RANCHER_CHARTS_REPO_DIR/release.yaml
index_entry=$(yq -o=j -I=0 $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION/Chart.yaml)
yq -i '.entries.rancher-turtles += '"$index_entry"'' $RANCHER_CHARTS_REPO_DIR/index.yaml
# Package the chart
$HELM package $RANCHER_CHARTS_REPO_DIR/charts/rancher-turtles/$RANCHER_CHART_DEV_VERSION --app-version=dev --version=$RANCHER_CHART_DEV_VERSION --destination=$RANCHER_CHARTS_REPO_DIR/assets/rancher-turtles
# Commit all changes
git -C $RANCHER_CHARTS_REPO_DIR config user.email "ci@rancher-turtles.local"
git -C $RANCHER_CHARTS_REPO_DIR config user.name "Rancher Turtles CI"
git -C $RANCHER_CHARTS_REPO_DIR add .
git -C $RANCHER_CHARTS_REPO_DIR commit -m "Added test chart $RANCHER_CHART_DEV_VERSION"
