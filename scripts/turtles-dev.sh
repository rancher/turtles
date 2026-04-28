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

if [ -z "$PANGOLIN_ENDPOINT" ]; then
    echo "You must pass a Pangolin endpoint. (ex. my.pangolin.example.com)"
    exit 1
fi

if [ -z "$NEWT_SITE_ID" ]; then
    echo "You must pass a Newt site client ID."
    exit 1
fi

if [ -z "$NEWT_SITE_SECRET" ]; then
    echo "You must pass a Newt site client secret."
    exit 1
fi

if [ -z "$PANGOLIN_SITE_DOMAIN" ]; then
    echo "You must pass a Pangolin site domain. (ex. myname.myteam.my.pangolin.example.com)"
    exit 1
fi

if [ -z "$PANGOLIN_SITE_IDENTIFIER" ]; then
    echo "You must pass a Pangolin site identifier. (ex. some-randomly-generated-name)"
    echo "Note: this is the **Identifier** of the Pangolin Site, not the name."
    echo "      This value is normally randomly generated when creating a new Site on the Pangolin Dashboard."
    exit 1
fi

RANCHER_HOSTNAME="rancher.$PANGOLIN_SITE_DOMAIN"
GITEA_HOSTNAME="gitea.$PANGOLIN_SITE_DOMAIN"
PANGOLIN_IP_ADDRESS=$(dig +short "$PANGOLIN_ENDPOINT")

RANCHER_CHANNEL=${RANCHER_CHANNEL:-alpha}
RANCHER_PASSWORD=${RANCHER_PASSWORD:-rancheradmin}
RANCHER_VERSION=${RANCHER_VERSION:-v2.14.0-alpha13}
RANCHER_IMAGE_TAG=${RANCHER_IMAGE_TAG:-$RANCHER_VERSION} # Set RANCHER_IMAGE_TAG=head to test with latest build
RANCHER_IMAGE=${RANCHER_IMAGE:-rancher/rancher:$RANCHER_IMAGE_TAG}
CLUSTER_NAME=${CLUSTER_NAME:-capi-test}
USE_TILT_DEV=${USE_TILT_DEV:-true}
TURTLES_VERSION=${TURTLES_VERSION:-dev}
TURTLES_IMAGE=${TURTLES_IMAGE:-ghcr.io/rancher/turtles:$TURTLES_VERSION}

GITEA_PASSWORD=${GITEA_PASSWORD:-giteaadmin}
RANCHER_CHARTS_REPO_DIR=${RANCHER_CHARTS_REPO_DIR}
RANCHER_CHART_DEV_VERSION=${RANCHER_CHART_DEV_VERSION}
RANCHER_CHARTS_BASE_BRANCH=${RANCHER_CHARTS_BASE_BRANCH}

RANCHER_CERT_DIR=${RANCHER_CERT_DIR:-/tmp/rancher-private-ca}
RANCHER_CERT_PATH=${RANCHER_CERT_PATH:-$RANCHER_CERT_DIR/tls.crt}
RANCHER_KEY_PATH=${RANCHER_KEY_PATH:-$RANCHER_CERT_DIR/tls.key}
RANCHER_CACERT_PATH=${RANCHER_CACERT_PATH:-$RANCHER_CERT_DIR/cacerts.pem}

BASEDIR=$(dirname "$0")

printf "\n\033[0;33m### Creating a management Cluster\033[0m\n"
kind create cluster --config "$BASEDIR/kind-cluster-with-extramounts.yaml" --name $CLUSTER_NAME
docker pull $RANCHER_IMAGE
kind load docker-image $RANCHER_IMAGE --name $CLUSTER_NAME

printf "\n\033[0;33m### Building and loading Turtles image\033[0m\n"
make docker-build-prime
kind load docker-image $TURTLES_IMAGE --name $CLUSTER_NAME

kubectl rollout status deployment coredns -n kube-system --timeout=90s

helm repo add rancher-$RANCHER_CHANNEL https://releases.rancher.com/server-charts/$RANCHER_CHANNEL --force-update
helm repo add gitea-charts https://dl.gitea.com/charts/ --force-update
helm repo add fossorial https://charts.fossorial.io --force-update
helm repo update

printf "\n\033[0;33m### Configuring Private CA Certificate\033[0m\n"
./scripts/create-rancher-certs.sh

# Create secrets
kubectl create namespace cattle-system
kubectl -n cattle-system create secret tls tls-rancher-ingress \
  --cert $RANCHER_CERT_PATH \
  --key $RANCHER_KEY_PATH
kubectl -n cattle-system create secret generic tls-ca \
  --from-file $RANCHER_CACERT_PATH

printf "\n\033[0;33m### Installing Gitea\033[0m\n"
helm install gitea gitea-charts/gitea \
    -f test/e2e/data/gitea/values.yaml \
    --set gitea.admin.password=$GITEA_PASSWORD \
    --namespace gitea \
    --create-namespace \
    --wait

# Deploy Gitea test Nodeport
kubectl apply -f test/e2e/data/gitea/test-nodeport.yaml

# Wait for Gitea to be accessible locally
echo "Waiting for Gitea to be accessible on localhost:30001..."
until curl -s -o /dev/null -w "%{http_code}" http://localhost:30001 | grep -q "200\|302\|301"; do 
    echo "Waiting for test Gitea Nodeport..."
    sleep 2
done
echo "Gitea is accessible locally!"

# Now setup Gitea repo and push charts
printf "\n\033[0;33m### Configuring new Gitea repository for Rancher charts\033[0m\n"
curl -X POST "http://gitea:$GITEA_PASSWORD@localhost:30001/api/v1/user/repos" \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    -d '{"name":"charts"}'

echo "Pushing charts repository to Gitea..."
git -C $RANCHER_CHARTS_REPO_DIR remote add fork http://gitea:$GITEA_PASSWORD@localhost:30001/gitea/charts.git
git -C $RANCHER_CHARTS_REPO_DIR push --set-upstream fork $RANCHER_CHARTS_BASE_BRANCH

printf "\n\033[0;33m### Installing Rancher\033[0m\n"
helm install rancher rancher-$RANCHER_CHANNEL/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword=$RANCHER_PASSWORD \
    --set replicas=1 \
    --set hostname="$RANCHER_HOSTNAME" \
    --set image.tag=$RANCHER_IMAGE_TAG \
    --set debug=true \
    --version="$RANCHER_VERSION" \
    --set ingress.tls.source=secret \
    --set privateCA=true \
    --set "extraEnv[0].name=CATTLE_CHART_DEFAULT_URL" \
    --set "extraEnv[0].value=http://gitea-http.gitea.svc.cluster.local:3000/gitea/charts.git" \
    --set "extraEnv[1].name=CATTLE_CHART_DEFAULT_BRANCH" \
    --set "extraEnv[1].value=$RANCHER_CHARTS_BASE_BRANCH" \
    --set "extraEnv[2].name=CATTLE_RANCHER_TURTLES_VERSION" \
    --set "extraEnv[2].value=$RANCHER_CHART_DEV_VERSION" \
    --wait

# Deploy Rancher test Nodeport
echo "Deploying Rancher test Nodeport..."
kubectl apply -f test/e2e/data/rancher/test-nodeport.yaml

# Wait for Rancher to be accessible locally
echo "Waiting for Rancher to be accessible on localhost:30080..."
until curl -s -o /dev/null -w "%{http_code}" http://localhost:30080 | grep -q "200\|302\|301"; do 
    echo "Waiting for test Rancher Nodeport..."
    sleep 2
done
echo "Rancher is accessible locally!"

printf "\n\033[0;33m### Installing Newt\033[0m\n"
kubectl create namespace newt
kubectl -n newt create secret generic newt-auth \
    --from-literal=NEWT_ID=$NEWT_SITE_ID \
    --from-literal=NEWT_SECRET=$NEWT_SITE_SECRET \
    --from-literal=PANGOLIN_ENDPOINT="https://$PANGOLIN_ENDPOINT"
mkdir -p /tmp/newt
PANGOLIN_IP_ADDRESS="$PANGOLIN_IP_ADDRESS" RANCHER_HOSTNAME="$RANCHER_HOSTNAME" GITEA_HOSTNAME="$GITEA_HOSTNAME" envsubst <test/e2e/data/newt/values.yaml > /tmp/newt/values.yaml
helm install newt fossorial/newt \
  -n newt \
  -f /tmp/newt/values.yaml \
  --wait

# Wait for both endpoints to be accessible via Pangolin
echo "Waiting for Gitea to be accessible via Pangolin (https://$GITEA_HOSTNAME)..."
RETRY_COUNT=0
MAX_RETRIES=30
until [ "$(curl -s -o /dev/null -w "%{http_code}" https://$GITEA_HOSTNAME)" = "200" ]; do 
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -gt $MAX_RETRIES ]; then
        echo "ERROR: Gitea not accessible via Pangolin after $MAX_RETRIES attempts"
        exit 1
    fi
    echo "Waiting for gitea via Pangolin (attempt $RETRY_COUNT/$MAX_RETRIES)..."
    sleep 2
done
echo "Gitea is accessible via Pangolin!"

echo "Waiting for Rancher to be accessible via Pangolin (https://$RANCHER_HOSTNAME)..."
RETRY_COUNT=0
until [ "$(curl -s -o /dev/null -w "%{http_code}" https://$RANCHER_HOSTNAME)" = "200" ] || [ "$(curl -s -o /dev/null -w "%{http_code}" https://$RANCHER_HOSTNAME)" = "302" ]; do 
    RETRY_COUNT=$((RETRY_COUNT+1))
    if [ $RETRY_COUNT -gt $MAX_RETRIES ]; then
        echo "ERROR: Rancher not accessible via Pangolin after $MAX_RETRIES attempts"
        exit 1
    fi
    echo "Waiting for rancher via Pangolin (attempt $RETRY_COUNT/$MAX_RETRIES)..."
    sleep 2
done
echo "Rancher is accessible via Pangolin!"

printf "\n\033[0;33m### Applying custom Rancher Settings\033[0m\n"
envsubst <test/e2e/data/rancher/rancher-setting-patch.yaml | kubectl apply -f -
kubectl apply -f test/e2e/data/rancher/system-store-setting-patch.yaml

# Wait for Rancher to restart with new config
kubectl rollout status deployment/rancher -n cattle-system --timeout=300s

install_local_providers_chart() {
    make build-providers-chart

    # Wait for Turtles to be ready. This may take a few minutes before Rancher installs the system chart.
    # The providers chart depends on CAPIProvider crd.
    kubectl wait --for=create crds/capiproviders.turtles-capi.cattle.io --timeout=300s

    helm upgrade --install rancher-turtles-providers out/charts/rancher-turtles-providers \
        -n cattle-turtles-system \
        --set providers.bootstrapRKE2.enabled=true \
        --set providers.bootstrapRKE2.manager.verbosity=5 \
        --set providers.controlplaneRKE2.enabled=true \
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

printf "\n\033[0;33m### Installing local Rancher Turtles Providers\033[0m\n"
install_local_providers_chart

printf "\n\033[0;33m### Exposing ETCD\033[0m\n"
kubectl apply -f test/e2e/data/etcd/test-nodeport.yaml
echo "To verify and collect ETCD data, run: make collect-etcd-data"

if [ "$USE_TILT_DEV" == "true" ]; then
    kubectl wait --for=create deployments/rancher-turtles-controller-manager --namespace cattle-turtles-system --timeout=300s
    printf "\n\033[0;33m### Using Tilt for development\033[0m\n"
    tilt up
fi
