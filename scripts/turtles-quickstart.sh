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

# Exit if a command fails
set -e

RANCHER_HOSTNAME=${RANCHER_HOSTNAME:-rancher.loc}
RANCHER_PASSWORD=${RANCHER_PASSWORD:-rancheradmin}
RANCHER_VERSION=${RANCHER_VERSION:-v2.10.2}
RANCHER_IMAGE=${RANCHER_IMAGE:-rancher/rancher:$RANCHER_VERSION}
RANCHER_CLUSTER_NAME=${RANCHER_CLUSTER_NAME:-rancher-cluster}
CAPI_CLUSTER_NAME=${CAPI_CLUSTER_NAME:-capi-cluster1}

BASEDIR=$(dirname "$0")

current_watches=$(sysctl -n fs.inotify.max_user_watches)
current_instances=$(sysctl -n fs.inotify.max_user_instances)

if [ "$current_watches" -lt 1048576 ] || [ "$current_instances" -lt 8192 ]; then
    echo
    echo "WARNING: This script requires increased inotify limits to function properly."
    echo "Please run the following commands:"
    echo ""
    echo "    sudo sysctl fs.inotify.max_user_watches=1048576"
    echo "    sudo sysctl fs.inotify.max_user_instances=8192"
    echo ""
    echo "To make these changes permanent, add these lines to /etc/sysctl.conf and run 'sudo sysctl -p'"
    echo ""

    echo "NOTICE: Your current settings are insufficient:"
    echo "Current max_user_watches: $current_watches (recommended: 1048576)"
    echo "Current max_user_instances: $current_instances (recommended: 8192)"
    echo ""
    
    read -p "Would you like to set these values now? (y/n): " answer
    if [[ "$answer" =~ ^[Yy]$ ]]; then
      sudo sysctl fs.inotify.max_user_watches=1048576
      sudo sysctl fs.inotify.max_user_instances=8192
      echo "Values have been updated for this session."
      echo "Add them to /etc/sysctl.conf to make them permanent."
    fi
fi

check_command() {
    if ! command -v $1 &> /dev/null 2>&1; then
        echo
        echo "$1 is not installed!"
        echo
        echo "Please install $1 first by following the instructions here:"
        echo "$2"
        exit 1
    fi
    echo "$1 checked - OK!"
}

show_step() {
    echo
    echo "===================================================" 
    echo "STEP: ${1}"
    echo "==================================================="
    echo
}

show_step "Checking if required tools are installed"

check_command kind https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-release-binaries
check_command kubectl https://kubernetes.io/docs/tasks/tools/#kubectl
check_command helm https://helm.sh/docs/intro/quickstart/#install-helm
check_command docker https://docs.docker.com/desktop/setup/install/linux/

show_step "Checking local kind ${RANCHER_CLUSTER_NAME}"

if kind get clusters | grep "$RANCHER_CLUSTER_NAME"; then
    echo
    echo "WARNING: There is already a kind cluster with the name $RANCHER_CLUSTER_NAME."
    echo
    echo

    read -p "Would you like to delete previous cluster now? (y/n): " answer
    if [[ "$answer" =~ ^[Yy]$ ]]; then
      echo "Removing previous cluster..."
      kind delete cluster --name $RANCHER_CLUSTER_NAME
      kind delete cluster --name $CAPI_CLUSTER_NAME
    else
      echo "Exiting..."
      exit 1
    fi
fi

show_step "Creating local kind ${RANCHER_CLUSTER_NAME} for Rancher"

cat > kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $RANCHER_CLUSTER_NAME
nodes:
- role: control-plane
  image: kindest/node:v1.30.0
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  extraMounts:
  - hostPath: /var/run/docker.sock
    containerPath: /var/run/docker.sock
EOF

kind create cluster --config "$BASEDIR/kind-config.yaml" --name $RANCHER_CLUSTER_NAME

docker pull $RANCHER_IMAGE
kind load docker-image $RANCHER_IMAGE --name $RANCHER_CLUSTER_NAME

kubectl rollout status deployment coredns -n kube-system --timeout=90s

kubectl cluster-info --context kind-$RANCHER_CLUSTER_NAME
kubectl wait --for=condition=Ready node/$RANCHER_CLUSTER_NAME-control-plane
kubectl get nodes -o wide

show_step "Install Ingress NGINX controller on ${RANCHER_CLUSTER_NAME}"

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install --create-namespace --namespace ingress-nginx ingress-nginx ingress-nginx/ingress-nginx --set controller.hostNetwork=true

show_step "Install Cert Manager on ${RANCHER_CLUSTER_NAME}"

helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --set crds.enabled=true

show_step "Install Rancher latest version on ${RANCHER_CLUSTER_NAME}"

helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo update
helm install rancher rancher-latest/rancher \
    --namespace cattle-system \
    --create-namespace \
    --set bootstrapPassword="$RANCHER_PASSWORD" \
    --set replicas=1 \
    --set hostname="$RANCHER_HOSTNAME" \
    --set ingress.ingressClassName=nginx \
    --version="$RANCHER_VERSION" \
    --wait

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

cat > rancher-settings.yaml <<EOF
apiVersion: management.cattle.io/v3
kind: Setting
metadata:
  name: server-url
value: https://${RANCHER_HOSTNAME}
---
apiVersion: management.cattle.io/v3
kind: Setting
metadata:
  name: first-login
value: "false"
---
apiVersion: management.cattle.io/v3
kind: Setting
metadata:
  name: eula-agreed
value: "2023-08-16T20:39:11.062Z"
---
apiVersion: management.cattle.io/v3
kind: Setting
metadata:
  name: agent-tls-mode
value: "system-store"
EOF

kubectl apply -f rancher-settings.yaml

kubectl rollout status deployment rancher -n cattle-system --timeout=180s

show_step "Waiting for Rancher to be READY ..."

until [[ $(curl -ks https://${RANCHER_HOSTNAME}/ping) == "pong" ]]; do
    sleep 1
done
echo "Received pong!"

show_step "Install Rancher Turtles on ${RANCHER_CLUSTER_NAME}"

helm repo add turtles https://rancher.github.io/turtles
helm repo update
helm install rancher-turtles turtles/rancher-turtles --version v0.17.0 \
    -n rancher-turtles-system \
    --set rancherTurtles.managerArguments={--insecure-skip-verify} \
    --set turtlesUI.enabled=true \
    --set turtlesUI.version=0.7.0 \
    --dependency-update \
    --create-namespace --wait \
    --timeout 180s

show_step "Waiting for deployment capi-controller-manager to be created..."

until kubectl get deployment capi-controller-manager -n capi-system > /dev/null 2>&1; do
  sleep 1
done
kubectl rollout status deployment capi-controller-manager -n capi-system --timeout=180s

# Enable the experimental Cluster topology feature.
export CLUSTER_TOPOLOGY=true

show_step "Install CAPI Docker provider on ${RANCHER_CLUSTER_NAME}"

clusterctl init --infrastructure docker

# The list of service CIDR, default ["10.128.0.0/12"]
export SERVICE_CIDR=["10.96.0.0/12"]

# The list of pod CIDR, default ["192.168.0.0/16"]
export POD_CIDR=["192.168.0.0/16"]

kubectl rollout status deployment capd-controller-manager -n capd-system --timeout=180s

show_step "Create a new example cluster using CAPI Docker provider"

kubectl create namespace capi-clusters
clusterctl generate cluster ${CAPI_CLUSTER_NAME} --flavor development \
    --kubernetes-version v1.30.0 \
    --control-plane-machine-count=1 \
    --worker-machine-count=1 \
    -n capi-clusters \
    > ${CAPI_CLUSTER_NAME}.yaml

kubectl apply -f ${CAPI_CLUSTER_NAME}.yaml

show_step "Waiting for the CAPI cluster to be provisioned ..."

SECONDS=0
TIMEOUT=240
until [[ $(kubectl get -n capi-clusters cluster ${CAPI_CLUSTER_NAME} -o jsonpath='{.status.phase}' 2>/dev/null) == "Provisioned" ]]; do
    [[ $SECONDS -ge $TIMEOUT ]] && echo "timeout reached" && break
    echo "Waiting for the cluster to be provisioned... ${SECONDS}s"
    sleep 1 
done
echo "Cluster ${CAPI_CLUSTER_NAME} was PROVISIONED by CAPI!"

show_step "Get the kubeconfig for the new cluster"

echo "clusterctl get kubeconfig -n capi-clusters ${CAPI_CLUSTER_NAME} > ${CAPI_CLUSTER_NAME}.kubeconfig"
until kind get kubeconfig -n ${CAPI_CLUSTER_NAME} &> /dev/null 2>&1; do
  sleep 1
done
clusterctl get kubeconfig -n capi-clusters ${CAPI_CLUSTER_NAME} > ${CAPI_CLUSTER_NAME}.kubeconfig

show_step "Deploying Calico CNI on the new example CAPI cluster"

kubectl --kubeconfig=./${CAPI_CLUSTER_NAME}.kubeconfig \
    apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml

kubectl label namespace capi-clusters cluster-api.cattle.io/rancher-auto-import=true

NODE_IP=$(kubectl get nodes -o jsonpath='{ $.items[*].status.addresses[?(@.type=="InternalIP")].address }')

echo
echo "SUCCESS: Rancher is ready at:"
echo "        https://$RANCHER_HOSTNAME"
echo
echo "Login: admin"
echo "Password: ${RANCHER_PASSWORD}"
echo
echo "NOTES: To get access to Rancher pleaese add to /etc/hosts a new line:"
echo "$NODE_IP rancher.loc"
echo
echo "NOTES: To access the new example CAPI cluster, use the following kubeconfig:"
echo "export KUBECONFIG=$HOME/.kube/config:$BASEDIR/${CAPI_CLUSTER_NAME}.kubeconfig"
echo
echo "kubectl config use-context ${CAPI_CLUSTER_NAME}-admin@${CAPI_CLUSTER_NAME}"
echo
