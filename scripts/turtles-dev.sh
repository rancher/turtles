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

set -x

RANCHER_HOSTNAME=$1
if [ -z "$RANCHER_HOSTNAME" ]; then
    echo "You must pass a rancher host name"
    exit 1
fi

RANCHER_CHANNEL=${RANCHER_CHANNEL:-latest}
RANCHER_PASSWORD=${RANCHER_PASSWORD:-rancheradmin}
RANCHER_VERSION=${RANCHER_VERSION:-v2.12.1}
RANCHER_IMAGE=${RANCHER_IMAGE:-rancher/rancher:$RANCHER_VERSION}
CLUSTER_NAME=${CLUSTER_NAME:-capi-test}
DAY2_CONTROLLER_IMAGE=${DAY2_CONTROLLER_IMAGE:-ghcr.io/rancher/turtles}
DAY2_CONTROLLER_IMAGE_TAG=${DAY2_CONTROLLER_IMAGE_TAG:-dev}
CLUSTERCLASS_CONTROLLER_IMAGE=${CLUSTERCLASS_CONTROLLER_IMAGE:-ghcr.io/rancher/turtles}
CLUSTERCLASS_CONTROLLER_IMAGE_TAG=${CLUSTERCLASS_CONTROLLER_IMAGE_TAG:-dev}
USE_TILT_DEV=${USE_TILT_DEV:-true}

BASEDIR=$(dirname "$0")

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml" --name $CLUSTER_NAME
docker pull $RANCHER_IMAGE
kind load docker-image $RANCHER_IMAGE --name $CLUSTER_NAME

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add rancher-$RANCHER_CHANNEL https://releases.rancher.com/server-charts/$RANCHER_CHANNEL --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo add ngrok https://charts.ngrok.com --force-update
helm repo update

helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set crds.enabled=true

helm upgrade ngrok ngrok/ngrok-operator \
    --namespace ngrok \
    --create-namespace \
    --install \
    --wait \
    --timeout 5m \
    --set credentials.apiKey=$NGROK_API_KEY \
    --set credentials.authtoken=$NGROK_AUTHTOKEN

helm install rancher rancher-$RANCHER_CHANNEL/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword=$RANCHER_PASSWORD \
    --set replicas=1 \
    --set hostname="$RANCHER_HOSTNAME" \
    --version="$RANCHER_VERSION" \
    --wait

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

kubectl apply -f test/e2e/data/rancher/ingress-class-patch.yaml
kubectl apply -f test/e2e/data/rancher/rancher-service-patch.yaml
envsubst <test/e2e/data/rancher/ingress.yaml | kubectl apply -f -
envsubst <test/e2e/data/rancher/rancher-setting-patch.yaml | kubectl apply -f -
kubectl apply -f test/e2e/data/rancher/system-store-setting-patch.yaml

# Install the locally build chart of Rancher Turtles
install_local_rancher_turtles_chart() {
    # Remove the previous chart directory
    rm -rf out
    # Build the chart locally
    make build-chart
    # Build the day2 controller image
    make docker-build
    # Load the day2 controller image into the kind cluster
    kind load docker-image $DAY2_CONTROLLER_IMAGE:$DAY2_CONTROLLER_IMAGE_TAG --name $CLUSTER_NAME
    # Load the clusterclass controller image into the kind cluster
    kind load docker-image $CLUSTERCLASS_CONTROLLER_IMAGE:$CLUSTERCLASS_CONTROLLER_IMAGE_TAG --name $CLUSTER_NAME
    # Install Rancher Turtles using a local chart with 'day2-operations' & 'clusterclass-operations'
    # feature flags enabled to run day2 & clusterclass controllers
    helm upgrade --install rancher-turtles out/charts/rancher-turtles \
        -n rancher-turtles-system \
        --set rancherTurtles.features.day2operations.enabled=true \
        --set rancherTurtles.features.day2operations.imageVersion=dev \
        --set rancherTurtles.features.day2operations.etcdBackupRestore.enabled=true \
        --set rancherTurtles.features.clusterclass-operations.enabled=true \
        --set rancherTurtles.features.clusterclass-operations.imageVersion=dev \
        --set rancherTurtles.imageVersion=dev \
        --dependency-update \
        --create-namespace --wait \
        --timeout 180s

    kubectl apply -f test/e2e/data/capi-operator/clusterctlconfig.yaml
}

echo "Installing local Rancher Turtles chart for development..."
install_local_rancher_turtles_chart

if [ "$USE_TILT_DEV" == "true" ]; then
    echo "Using Tilt for development..."
    tilt up
fi
