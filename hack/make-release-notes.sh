#!/usr/bin/env bash

set -e
set -o errexit
set -o pipefail

TOOLS_FOLDER=${1:-hack/tools/bin}

git clone -n https://github.com/kubernetes-sigs/cluster-api.git --depth 1
cd cluster-api
git checkout a0f2568 -- hack/tools/release/notes.go
go build hack/tools/release/notes.go 
mv notes $TOOLS_FOLDER
cd .. && rm -rf cluster-api