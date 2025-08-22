#!/usr/bin/env bash

# Copyright © 2025 SUSE LLC
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

RANCHER_VERSION=${RANCHER_VERSION:-v2.11.3}
RANCHER_IMAGE=${RANCHER_IMAGE:-rancher/rancher:$RANCHER_VERSION}
RANCHER_CLUSTER_NAME=${RANCHER_CLUSTER_NAME:-rancher-cluster}
RANCHER_TURTLES_VERSION=${RANCHER_TURTLES_VERSION:-v0.22.0}
CAPI_CLUSTER_NAME=${CAPI_CLUSTER_NAME:-capi-cluster1}
KIND_IMAGE_VERSION=${KIND_IMAGE_VERSION:-v1.31.6}

CONFIG_DIR="/tmp/turtles"
mkdir -p $CONFIG_DIR

WITH_NGROK=0
WITH_EXAMPLE=0
WITHOUT_RANCHER=0

# Function to display the usage information
usage() {
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --help              Show urtles-quickstart script usage"
    echo "  --with-example      Deploy example CAPI cluster with Docker provider" 
    echo "  --without-rancher   Do not install local Rancher with kind along with Turtles"
    echo "  --with-ngrok        Use NGROK as Ingress controller for Rancher"
    echo "                      NGROK_API_KEY and NGROK_AUTHTOKEN environment variables must be set"
    echo "                      NGROK_API_KEY=<> NGROK_AUTHTOKEN=<> ./turtles-quickstart.sh --with-ngrok"
    echo
    exit 1
}

# Process each argument
for arg in "$@"; do
    case "$arg" in
        --help)
            usage
            ;;
        --with-ngrok)
            WITH_NGROK=1
            ;;
        --with-example)
            WITH_EXAMPLE=1
            ;;
        --without-rancher)
            echo
            echo "Installing Rancher Turtles without Rancher"
            WITHOUT_RANCHER=1
            ;;
        *)
            echo "Unknown option: $arg"
            usage
            ;;
    esac
done

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

#### script starts here

if [ "$WITHOUT_RANCHER" -eq 0 ]; then
    # Check if RANCHER_HOSTNAME is not set, ask the user
    if [ -z "$RANCHER_HOSTNAME" ]; then
        read -p "Enter Rancher hostname [default: rancher.loc]: " RANCHER_HOSTNAME_INPUT
        RANCHER_HOSTNAME=${RANCHER_HOSTNAME_INPUT:-rancher.loc}
    else
        echo "Using provided RANCHER_HOSTNAME: $RANCHER_HOSTNAME"
    fi
    # Check if RANCHER_PASSWORD is not set, ask the user
    if [ -z "$RANCHER_PASSWORD" ]; then
        read -p "Enter Rancher admin password [default: rancheradmin]: " RANCHER_PASSWORD_INPUT
        RANCHER_PASSWORD=${RANCHER_PASSWORD_INPUT:-rancheradmin}
    else
        echo "Using provided RANCHER_PASSWORD: $RANCHER_PASSWORD"
    fi

    current_watches=$(sysctl -n fs.inotify.max_user_watches)
    current_instances=$(sysctl -n fs.inotify.max_user_instances)

    if [ "$current_watches" -lt 1048576 ] || [ "$current_instances" -lt 8192 ]; then
        echo
        echo "WARNING: When creating many nodes using Cluster API and Docker infrastructure,"
        echo "the OS may run into inotify limits which prevent new nodes from being provisioned." 
        echo
        echo "More information:"
        echo "https://cluster-api.sigs.k8s.io/user/troubleshooting.html?highlight=inoti#cluster-api-with-docker----too-many-open-files"
        echo
        echo "This script requires increased inotify limits to function properly."
        echo "Please run the following commands:"
        echo
        echo "    sudo sysctl fs.inotify.max_user_watches=1048576"
        echo "    sudo sysctl fs.inotify.max_user_instances=8192"
        echo
        echo "To make these changes permanent, add these lines to /etc/sysctl.conf and run 'sudo sysctl -p'"
        echo
        echo "NOTICE: Your current settings are insufficient:"
        echo "Current max_user_watches: $current_watches (recommended: 1048576)"
        echo "Current max_user_instances: $current_instances (recommended: 8192)"
        echo
        
        read -p "Would you like to set these values now? (y/n): " answer
        if [[ "$answer" =~ ^[Yy]$ ]]; then
          sudo sysctl fs.inotify.max_user_watches=1048576
          sudo sysctl fs.inotify.max_user_instances=8192
          echo "Values have been updated for this session."
          echo "Add them to /etc/sysctl.conf to make them permanent."
        fi
    fi

    show_step "Checking if required tools are installed"

    check_command kind https://kind.sigs.k8s.io/docs/user/quick-start/#installing-from-release-binaries
    check_command kubectl https://kubernetes.io/docs/tasks/tools/#kubectl
    check_command helm https://helm.sh/docs/intro/quickstart/#install-helm
    check_command docker https://docs.docker.com/engine/install/

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

    cat > ${CONFIG_DIR}/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $RANCHER_CLUSTER_NAME
nodes:
- role: control-plane
  image: kindest/node:${KIND_IMAGE_VERSION}
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

    kind create cluster --config "${CONFIG_DIR}/kind-config.yaml" --name $RANCHER_CLUSTER_NAME

    kubectl rollout status deployment coredns -n kube-system --timeout=90s

    kubectl cluster-info --context kind-$RANCHER_CLUSTER_NAME
    kubectl wait --for=condition=Ready node/$RANCHER_CLUSTER_NAME-control-plane
    kubectl get nodes -o wide

    if [ "$WITH_NGROK" -eq 1 ]; then
        show_step "Install NGROK controller on ${RANCHER_CLUSTER_NAME}"

        helm repo add ngrok https://charts.ngrok.com --force-update
        helm repo update
        helm upgrade ngrok ngrok/ngrok-operator \
          --namespace ngrok \
          --create-namespace \
          --install \
          --wait \
          --timeout 5m \
          --set credentials.apiKey=$NGROK_API_KEY \
          --set credentials.authtoken=$NGROK_AUTHTOKEN
        INGRESS_CLASS="ngrok"
    else 
        show_step "Install Ingress NGINX controller on ${RANCHER_CLUSTER_NAME}"

        helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
        helm repo update
        helm install ingress-nginx ingress-nginx/ingress-nginx \
            --namespace ingress-nginx \
            --create-namespace \
            --set controller.hostNetwork=true
        INGRESS_CLASS="nginx"
    fi

    show_step "Install Cert Manager on ${RANCHER_CLUSTER_NAME}"

    helm repo add jetstack https://charts.jetstack.io
    helm repo update
    helm install cert-manager jetstack/cert-manager \
        --namespace cert-manager \
        --create-namespace \
        --set crds.enabled=true

    show_step "Install Rancher ${RANCHER_VERSION} on ${RANCHER_CLUSTER_NAME}"

    docker pull $RANCHER_IMAGE
    kind load docker-image $RANCHER_IMAGE --name $RANCHER_CLUSTER_NAME

    helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
    helm repo update
    helm install rancher rancher-latest/rancher \
        --namespace cattle-system \
        --create-namespace \
        --set bootstrapPassword="$RANCHER_PASSWORD" \
        --set replicas=1 \
        --set hostname="$RANCHER_HOSTNAME" \
        --set ingress.ingressClassName="$INGRESS_CLASS" \
        --version="$RANCHER_VERSION" \
        --wait

    kubectl rollout status deployment rancher -n cattle-system --timeout=180s

    cat << EOF | kubectl apply -f -
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
EOF

    if [ "$WITH_NGROK" -eq 1 ]; then

    show_step "Update Rancher service for NGROK"

    cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  annotations:
    k8s.ngrok.com/app-protocols: '{"https-internal":"HTTPS","http":"HTTP"}'
  name: rancher
  namespace: cattle-system
EOF

    show_step "Using Rancher agent-tls-mode SystemStore setting"

    cat << EOF | kubectl apply -f -
apiVersion: management.cattle.io/v3
kind: Setting
metadata:
  name: agent-tls-mode
value: "system-store"
EOF
    fi 

    kubectl rollout status deployment rancher -n cattle-system --timeout=180s

    show_step "Waiting for Rancher to be READY ..."

    if [ "$WITH_NGROK" -eq 1 ]; then
        until [[ $(curl -ks https://${RANCHER_HOSTNAME}/ping) == "pong" ]]; do
            sleep 1
        done
    else
        NODE_IP=$(kubectl get nodes -o jsonpath='{ $.items[*].status.addresses[?(@.type=="InternalIP")].address }')

        until [[ $(curl -ks -H "Host: ${RANCHER_HOSTNAME}" https://${NODE_IP}/ping) == "pong" ]]; do
            sleep 1
        done
    fi

    echo "Received pong!"
fi

show_step "Install Rancher Turtles on ${RANCHER_CLUSTER_NAME}"

helm repo add turtles https://rancher.github.io/turtles
helm repo update
helm install rancher-turtles turtles/rancher-turtles --version ${RANCHER_TURTLES_VERSION} \
    -n rancher-turtles-system \
    --set turtlesUI.enabled=true \
    --dependency-update \
    --create-namespace --wait \
    --timeout 180s

show_step "Waiting for deployment capi-controller-manager to be created..."

until kubectl get deployment capi-controller-manager -n capi-system > /dev/null 2>&1; do
  sleep 1
done
kubectl rollout status deployment capi-controller-manager -n capi-system --timeout=180s


if [ "$WITH_EXAMPLE" -eq 1 ]; then
    # Enable the experimental Cluster topology feature.
    export CLUSTER_TOPOLOGY=true

    show_step "Install CAPI Docker provider on ${RANCHER_CLUSTER_NAME}"

    # Install the CAPI Docker provider
    cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: capd-system
---
apiVersion: turtles-capi.cattle.io/v1alpha1
kind: CAPIProvider
metadata:
  name: docker
  namespace: capd-system
spec:
  type: infrastructure
EOF

    # Install kubeadm bootstrap  and control-plane providers
    cat << EOF | kubectl apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: capi-kubeadm-bootstrap-system
---
apiVersion: turtles-capi.cattle.io/v1alpha1
kind: CAPIProvider
metadata:
  name: kubeadm-bootstrap
  namespace: capi-kubeadm-bootstrap-system
spec:
  name: kubeadm
  type: bootstrap
---
apiVersion: v1
kind: Namespace
metadata:
  name: capi-kubeadm-control-plane-system
---
apiVersion: turtles-capi.cattle.io/v1alpha1
kind: CAPIProvider
metadata:
  name: kubeadm-control-plane
  namespace: capi-kubeadm-control-plane-system
spec:
  name: kubeadm
  type: controlPlane
EOF

    until kubectl get deployment capd-controller-manager -n capd-system > /dev/null 2>&1; do
      sleep 1
    done
    # Wait for all providers to be ready
    kubectl -n rke2-control-plane-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-control-plane-controller-manager
    kubectl -n rke2-bootstrap-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-bootstrap-controller-manager
    kubectl -n capd-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capd-controller-manager
    kubectl -n capi-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capi-controller-manager
    kubectl rollout status deployment capd-controller-manager -n capd-system --timeout=180s

    sleep 5
    show_step "Create a new example cluster using CAPI Docker provider"

    kubectl create namespace capi-clusters

    export CLUSTER_NAME=${CAPI_CLUSTER_NAME}
    export CLUSTER_CLASS_NAME=docker-cc
    export NAMESPACE=capi-clusters
    export CONTROL_PLANE_MACHINE_COUNT=1
    export WORKER_MACHINE_COUNT=1
    export KUBERNETES_VERSION=v1.30.4

    curl -s https://raw.githubusercontent.com/rancher/turtles/refs/heads/main/test/e2e/data/cluster-templates/docker-kubeadm.yaml | envsubst > ${CONFIG_DIR}/${CAPI_CLUSTER_NAME}.yaml

    kubectl apply -f ${CONFIG_DIR}/${CAPI_CLUSTER_NAME}.yaml

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

    echo "kind get kubeconfig --name ${CAPI_CLUSTER_NAME} > ${CAPI_CLUSTER_NAME}.kubeconfig"
    until kind get kubeconfig --name ${CAPI_CLUSTER_NAME} &> /dev/null 2>&1; do
      sleep 1
    done
    kind get kubeconfig --name ${CAPI_CLUSTER_NAME} > ${CAPI_CLUSTER_NAME}.kubeconfig

    show_step "Deploying Calico CNI on the new example CAPI cluster"

    kubectl --kubeconfig=./${CAPI_CLUSTER_NAME}.kubeconfig \
        apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.26.1/manifests/calico.yaml

    kubectl label namespace capi-clusters cluster-api.cattle.io/rancher-auto-import=true
fi

echo
echo "SUCCESS: Turtles installation completed successfully!"
echo

if [ "$WITHOUT_RANCHER" -eq 0 ]; then
    echo "Rancher is ready at:"
    echo "        https://$RANCHER_HOSTNAME"
    echo
    echo "Login: admin"
    echo "Password: ${RANCHER_PASSWORD}"
    echo
fi

if [ "$WITH_NGROK" -eq 0 ]; then
    echo
    echo "NOTES: To get access to Rancher please add to /etc/hosts a new line:"
    echo "$NODE_IP $RANCHER_HOSTNAME"
    echo
fi

if [ "$WITH_EXAMPLE" -eq 1 ]; then
    echo "NOTES: To access the new example CAPI cluster, use the following kubeconfig:"
    echo "export KUBECONFIG=$HOME/.kube/config:$CONFIG_DIR/${CAPI_CLUSTER_NAME}.kubeconfig"
    echo
    echo "kubectl config use-context ${CAPI_CLUSTER_NAME}-admin@${CAPI_CLUSTER_NAME}"
    echo
fi
