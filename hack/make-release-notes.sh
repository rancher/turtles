#!/usr/bin/env bash

# Copyright 2023 SUSE.
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

mkdir $TOOLS_FOLDER/cluster-api
cd $TOOLS_FOLDER/cluster-api
git init
git remote add origin https://github.com/kubernetes-sigs/cluster-api.git 
git fetch --depth 1 origin ce045ad2cddf21738799fcc310e847709b8ee41e
git checkout FETCH_HEAD -- hack/tools/release/notes.go
go build hack/tools/release/notes.go 
mv notes $TOOLS_FOLDER
cd - && rm -rf $TOOLS_FOLDER/cluster-api