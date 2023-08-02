#!/usr/bin/env bash

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