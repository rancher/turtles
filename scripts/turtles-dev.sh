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

set -xe

RANCHER_HOSTNAME=$1
if [ -z "$RANCHER_HOSTNAME" ]; then
    echo "You must pass a rancher host name"
    exit 1
fi

RANCHER_CHANNEL=${RANCHER_CHANNEL:-latest}
RANCHER_PASSWORD=${RANCHER_PASSWORD:-rancheradmin}
RANCHER_VERSION=${RANCHER_VERSION:-v2.13.0-rc1}
RANCHER_IMAGE_TAG=${RANCHER_IMAGE_TAG:-$RANCHER_VERSION} # Set RANCHER_IMAGE_TAG=head to test with latest build
RANCHER_IMAGE=${RANCHER_IMAGE:-rancher/rancher:$RANCHER_IMAGE_TAG}
CLUSTER_NAME=${CLUSTER_NAME:-capi-test}
USE_TILT_DEV=${USE_TILT_DEV:-true}
TURTLES_VERSION=${TURTLES_VERSION:-dev}
TURTLES_IMAGE=${TURTLES_IMAGE:-ghcr.io/rancher/turtles:$TURTLES_VERSION}

GITEA_PASSWORD=${GITEA_PASSWORD:-giteaadmin}
GITEA_INGRESS_CLASS_NAME=${GITEA_INGRESS_CLASS_NAME:-ngrok}
RANCHER_CHARTS_REPO_DIR=${RANCHER_CHARTS_REPO_DIR}
RANCHER_CHART_DEV_VERSION=${RANCHER_CHART_DEV_VERSION}
RANCHER_CHARTS_BASE_BRANCH=${RANCHER_CHARTS_BASE_BRANCH}

BASEDIR=$(dirname "$0")

if pgrep -x ngrok > /dev/null; then
    echo "Stopping existing ngrok processes..."
    pkill -x ngrok
    sleep 2
fi

kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml" --name $CLUSTER_NAME
docker pull $RANCHER_IMAGE
kind load docker-image $RANCHER_IMAGE --name $CLUSTER_NAME

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add rancher-$RANCHER_CHANNEL https://releases.rancher.com/server-charts/$RANCHER_CHANNEL --force-update
helm repo add jetstack https://charts.jetstack.io --force-update
helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update
helm repo update

helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set crds.enabled=true

helm install gitea gitea-charts/gitea \
    -f test/e2e/data/gitea/values.yaml \
    --set gitea.admin.password=$GITEA_PASSWORD \
    --namespace gitea \
    --create-namespace \
    --wait

cleanup() {
    echo "Cleaning up background processes..."
    [ -n "$NGROK_PID" ] && kill $NGROK_PID 2>/dev/null && echo "Stopped ngrok (PID: $NGROK_PID)"
    [ -f "$NGROK_CONFIG_FILE" ] && rm -f "$NGROK_CONFIG_FILE" && echo "Removed ngrok config file"
}
trap cleanup EXIT INT TERM

# Build and load the controller image
make docker-build-prime
kind load docker-image $TURTLES_IMAGE --name $CLUSTER_NAME

# Deploy Gitea test Nodeport
echo "Deploying Gitea test Nodeport..."
kubectl apply -f test/e2e/data/gitea/test-nodeport.yaml

# Wait for Gitea to be accessible locally
echo "Waiting for Gitea to be accessible on localhost:30001..."
until curl -s -o /dev/null -w "%{http_code}" http://localhost:30001 | grep -q "200\|302\|301"; do 
    echo "Waiting for test Gitea Nodeport..."
    sleep 2
done
echo "Gitea is accessible locally!"

# Now setup Gitea repo and push charts
echo "Creating new Gitea repository..."
curl -X POST "http://gitea:$GITEA_PASSWORD@localhost:30001/api/v1/user/repos" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    -d '{"name":"charts"}'

echo "Pushing charts repository to Gitea..."
git -C $RANCHER_CHARTS_REPO_DIR remote add fork http://gitea:$GITEA_PASSWORD@localhost:30001/gitea/charts.git
git -C $RANCHER_CHARTS_REPO_DIR push --set-upstream fork $RANCHER_CHARTS_BASE_BRANCH

helm install rancher rancher-$RANCHER_CHANNEL/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword=$RANCHER_PASSWORD \
    --set replicas=1 \
    --set hostname="$RANCHER_HOSTNAME" \
    --set image.tag=$RANCHER_IMAGE_TAG \
    --set debug=true \
    --version="$RANCHER_VERSION" \
    --wait

# Deploy Rancher test Nodeport
echo "Deploying Rancher test Nodeport..."
kubectl apply -f test/e2e/data/rancher/test-nodeport.yaml

# Wait for Rancher to be accessible locally
echo "Waiting for Rancher to be accessible on localhost:30002..."
until curl -s -o /dev/null -w "%{http_code}" http://localhost:30002 | grep -q "200\|302\|301"; do 
    echo "Waiting for test Rancher Nodeport..."
    sleep 2
done
echo "Rancher is accessible locally!"

# Now both services are ready, start ngrok with both endpoints
NGROK_CONFIG_FILE="/tmp/ngrok-turtles-dev.yml"
cat > "$NGROK_CONFIG_FILE" <<EOF
version: 2
authtoken: $NGROK_AUTHTOKEN
tunnels:
  rancher:
    proto: http
    addr: http://localhost:30002
    hostname: $RANCHER_HOSTNAME
  gitea:
    proto: http
    addr: http://localhost:30001
    hostname: gitea.$RANCHER_HOSTNAME
EOF

echo "Starting ngrok with both Rancher and Gitea endpoints..."
ngrok start --all --config "$NGROK_CONFIG_FILE" --log stdout > /tmp/ngrok-turtles-dev.log 2>&1 &
NGROK_PID=$!
echo "ngrok started with PID: $NGROK_PID"
sleep 5

if ! kill -0 $NGROK_PID 2>/dev/null; then
    echo "ERROR: ngrok failed, check logs:"
    cat /tmp/ngrok-turtles-dev.log
    exit 1
fi

# Wait for both endpoints to be accessible via ngrok
echo "Waiting for Gitea to be accessible via ngrok (https://gitea.$RANCHER_HOSTNAME)..."
RETRY_COUNT=0
MAX_RETRIES=30
until [ "$(curl -s -o /dev/null -w "%{http_code}" https://gitea.$RANCHER_HOSTNAME)" = "200" ]; do 
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -gt $MAX_RETRIES ]; then
        echo "ERROR: Gitea not accessible via ngrok after $MAX_RETRIES attempts"
        echo "ngrok logs:"
        cat /tmp/ngrok-turtles-dev.log
        exit 1
    fi
    echo "Waiting for gitea via ngrok (attempt $RETRY_COUNT/$MAX_RETRIES)..."
    sleep 2
done
echo "Gitea is accessible via ngrok!"

echo "Waiting for Rancher to be accessible via ngrok (https://$RANCHER_HOSTNAME)..."
RETRY_COUNT=0
until [ "$(curl -s -o /dev/null -w "%{http_code}" https://$RANCHER_HOSTNAME)" = "200" ] || [ "$(curl -s -o /dev/null -w "%{http_code}" https://$RANCHER_HOSTNAME)" = "302" ]; do 
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -gt $MAX_RETRIES ]; then
        echo "ERROR: Rancher not accessible via ngrok after $MAX_RETRIES attempts"
        echo "ngrok logs:"
        cat /tmp/ngrok-turtles-dev.log
        exit 1
    fi
    echo "Waiting for rancher via ngrok (attempt $RETRY_COUNT/$MAX_RETRIES)..."
    sleep 2
done
echo "Rancher is accessible via ngrok!"

envsubst <test/e2e/data/rancher/rancher-setting-patch.yaml | kubectl apply -f -
kubectl apply -f test/e2e/data/rancher/system-store-setting-patch.yaml

# Update Rancher deployment with environment variables pointing to Gitea charts
kubectl set env deployment/rancher -n cattle-system \
    CATTLE_CHART_DEFAULT_URL=https://gitea.$RANCHER_HOSTNAME/gitea/charts.git \
    CATTLE_CHART_DEFAULT_BRANCH=$RANCHER_CHARTS_BASE_BRANCH \
    CATTLE_RANCHER_TURTLES_VERSION=$RANCHER_CHART_DEV_VERSION

# Wait for Rancher to restart with new config
kubectl rollout status deployment/rancher -n cattle-system --timeout=300s

install_local_providers_chart() {
    make build-providers-chart

    # Wait for Turtles to be ready. This may take a few minutes before Rancher installs the system chart.
    # The providers chart depends on CAPIProvider crd.
    kubectl wait --for=create crds/capiproviders.turtles-capi.cattle.io --timeout=300s

    helm upgrade --install rancher-turtles-providers out/charts/rancher-turtles-providers \
        -n cattle-turtles-system \
        --set providers.bootstrapRKE2.manager.verbosity=5 \
        --set providers.controlplaneRKE2.manager.verbosity=5 \
        --set providers.bootstrapKubeadm.enabled=true \
        --set providers.bootstrapKubeadm.manager.verbosity=5 \
        --set providers.controlplaneKubeadm.enabled=true \
        --set providers.controlplaneKubeadm.manager.verbosity=5 \
        --set providers.infrastructureDocker.enabled=true \
        --set providers.infrastructureDocker.manager.verbosity=5 \
        --set providers.infrastructureAWS.enabled=true \
        --set providers.infrastructureAWS.manager.verbosity=5 \
        --set providers.infrastructureAzure.enabled=true \
        --set providers.infrastructureAzure.manager.verbosity=5 \
        --set providers.infrastructureGCP.enabled=true \
        --set providers.infrastructureGCP.manager.verbosity=5 \
        --set providers.infrastructureGCP.variables.GCP_B64ENCODED_CREDENTIALS="" \
        --set providers.infrastructureVSphere.enabled=true \
        --set providers.infrastructureVSphere.manager.verbosity=5 \
        --create-namespace --wait \
        --timeout 180s
}

echo "Installing local Rancher Turtles Providers..."
install_local_providers_chart

if [ "$USE_TILT_DEV" == "true" ]; then
    kubectl wait --for=create deployments/rancher-turtles-controller-manager --namespace cattle-turtles-system --timeout=300s
    echo "Using Tilt for development..."
    tilt up
fi
