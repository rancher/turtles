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

RANCHER_HOSTNAME=$1
if [ -z "$RANCHER_HOSTNAME" ]; then
	echo "You must pass a rancher host name"
	exit 1
fi

RANCHER_VERSION=${RANCHER_VERSION:-v2.8.2}

BASEDIR=$(dirname "$0")

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml"

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add rancher-stable https://releases.rancher.com/server-charts/stable
helm repo add capi-operator https://kubernetes-sigs.github.io/cluster-api-operator
helm repo update

export EXP_CLUSTER_RESOURCE_SET=true
export CLUSTER_TOPOLOGY=true

helm install capi-operator capi-operator/cluster-api-operator \
	--create-namespace -n capi-operator-system \
	--set infrastructure=docker:v1.4.6 \
	--set core=cluster-api:v1.4.6 \
	--set cert-manager.enabled=true \
	--timeout 90s --wait

kubectl rollout status deployment capi-operator-cluster-api-operator -n capi-operator-system --timeout=180s

helm install rancher rancher-stable/rancher \
	--namespace cattle-system \
	--create-namespace \
	--set bootstrapPassword=rancheradmin \
	--set replicas=1 \
	--set hostname="$RANCHER_HOSTNAME" \
	--set 'extraEnv[0].name=CATTLE_FEATURES' \
	--set 'extraEnv[0].value=embedded-cluster-api=false' \
	--version="$RANCHER_VERSION" \
	--wait

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

tilt up
