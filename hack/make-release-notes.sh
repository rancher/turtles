#!/usr/bin/env bash

# Copyright © 2023 - 2024 SUSE LLC
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

set -e
set -o errexit
set -o pipefail

TOOLS_FOLDER=${1:-hack/tools/bin}
CAPI_VERSION=${2}

mkdir $TOOLS_FOLDER/cluster-api
cd $TOOLS_FOLDER/cluster-api
git init
git remote add origin https://github.com/kubernetes-sigs/cluster-api.git 
git fetch --depth 1 origin tag $CAPI_VERSION
git checkout FETCH_HEAD -- hack/tools
go build -C hack/tools -o $TOOLS_FOLDER/notes -tags tools sigs.k8s.io/cluster-api/hack/tools/release/notes
cd - && rm -rf $TOOLS_FOLDER/cluster-api
