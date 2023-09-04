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

RANCHER_HOSTNAME=$1
if [ -z "$RANCHER_HOSTNAME" ]
then
    echo "You must pass a rancher host name"
    exit 1
fi

BASEDIR=$(dirname "$0")

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml"

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add jetstack https://charts.jetstack.io
helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo update

helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.12.3 \
    --set installCRDs=true \
    --wait

kubectl rollout status deployment cert-manager -n cert-manager --timeout=90s

export EXP_CLUSTER_RESOURCE_SET=true
export CLUSTER_TOPOLOGY=true
clusterctl init -i docker

helm install rancher rancher-stable/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword=rancheradmin \
    --set replicas=1 \
    --set hostname="$RANCHER_HOSTNAME" \
    --set 'extraEnv[0].name=CATTLE_FEATURES' \
    --set 'extraEnv[0].value=embedded-cluster-api=false' \
    --set global.cattle.psp.enabled=false \
    --version v2.7.5 \
    --wait

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

tilt up

