#!/bin/bash

# Copyright © 2024 - 2025 SUSE LLC
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

# This script is an example on how to provision a self-managed RKE2 Rancher cluster.
#
# One bootstrap cluster is provisioned using kind, and initialized with the required CAPI providers.
# Using the bootstrap cluster, a downstream RKE2+CAPD cluster is provisioned.
# Note that CAPD is used for demo purposes.
# Rancher, Turtles, the required CAPI providers and all dependencies are installed on the downstream cluster.
# Finally the downstream cluster is moved to itself, for self-management.
# For more information see: https://cluster-api.sigs.k8s.io/clusterctl/commands/move

set -e

CAPI_VERSION="v1.9.3"
CAPD_VERSION="v1.9.3"
CAPRKE2_VERSION="v0.10.0"

# Initialize a bootstrap CAPI cluster
cat << EOF | kind create cluster --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: bootstrap-capi-management
nodes:
- role: control-plane
  image: kindest/node:v1.31.4
  extraMounts:
    - hostPath: /var/run/docker.sock
      containerPath: /var/run/docker.sock
EOF

# Create a temporary clusterctl config
CONFIG_DIR="/tmp/cluster-api"
CONFIG_FILE="$CONFIG_DIR/clusterctl.yaml"
mkdir -p $CONFIG_DIR
cat << EOF > $CONFIG_FILE
providers:
- name: "rke2"
  url: "https://github.com/rancher/cluster-api-provider-rke2/releases/$CAPRKE2_VERSION/bootstrap-components.yaml"
  type: "BootstrapProvider"
- name: "rke2"
  url: "https://github.com/rancher/cluster-api-provider-rke2/releases/$CAPRKE2_VERSION/control-plane-components.yaml"
  type: "ControlPlaneProvider"
EOF

# Level 5 is highest for debugging
export CLUSTERCTL_LOG_LEVEL=4

# Install core, docker, and RKE2 providers
clusterctl init --config "$CONFIG_FILE" \
                --core "cluster-api:$CAPI_VERSION" \
                --bootstrap "rke2:$CAPRKE2_VERSION" --control-plane "rke2:$CAPRKE2_VERSION" \
                --infrastructure "docker:$CAPD_VERSION"

# Wait for all providers to be ready
kubectl -n rke2-control-plane-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-control-plane-controller-manager
kubectl -n rke2-bootstrap-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-bootstrap-controller-manager
kubectl -n capd-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capd-controller-manager
kubectl -n capi-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capi-controller-manager

# Bootstrap a downstream RKE2 cluster
cd "$(dirname "$0")"
kubectl apply -f ./bootstrap-cluster.yaml
kubectl wait --for=condition=Ready clusters/rancher --timeout=300s

# Fetch the downstream kubeconfig and switch to it
KUBECONFIG_FILE="/tmp/cluster-api/rancher"
clusterctl get kubeconfig rancher > "$KUBECONFIG_FILE"
chmod 600 "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"

# Install Rancher, Turtles, and dependencies on the downstream cluster
helm repo add jetstack https://charts.jetstack.io
helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
helm repo add turtles https://rancher.github.io/turtles
helm repo update

helm install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace \
    --set crds.enabled=true \
    --wait

# Note kubectl wait --for=create requires kubectl >= v1.31.x
kubectl -n kube-system wait --for=create daemonsets/rke2-ingress-nginx-controller
kubectl -n kube-system wait --for=jsonpath='{.status.numberReady}'=2 daemonsets/rke2-ingress-nginx-controller --timeout=300s
helm install rancher rancher-latest/rancher \
    --namespace cattle-system --create-namespace \
    --set bootstrapPassword=rancheradmin \
    --set replicas=1 \
    --set hostname="my.rancher.example.com" \
    --wait

helm install rancher-turtles turtles/rancher-turtles \
    -n rancher-turtles-system  --create-namespace \
    --dependency-update \
    --set cluster-api-operator.cluster-api.version="$CAPI_VERSION" \
    --set cluster-api-operator.cluster-api.rke2.version="$CAPRKE2_VERSION" \
    --wait --timeout 180s

# Install CAPD provider on downstream cluster
cat << EOF | kubectl apply -f -
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
  name: docker
  version: $CAPD_VERSION
EOF

# Wait for all providers to be ready on downstream cluster
kubectl -n rke2-control-plane-system wait --for=create deployments/rke2-control-plane-controller-manager --timeout=300s
kubectl -n rke2-control-plane-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-control-plane-controller-manager
kubectl -n rke2-bootstrap-system wait --for=create deployments/rke2-bootstrap-controller-manager --timeout=300s
kubectl -n rke2-bootstrap-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/rke2-bootstrap-controller-manager
kubectl -n capd-system wait --for=create deployments/capd-controller-manager --timeout=300s
kubectl -n capd-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capd-controller-manager
kubectl -n capi-system wait --for=create deployments/capi-controller-manager --timeout=300s
kubectl -n capi-system wait --for=jsonpath='{.status.readyReplicas}'=1 deployments/capi-controller-manager

# Pivot the CAPI cluster to the newly provisioned downstream cluster
capioperator move --to-kubeconfig="$KUBECONFIG_FILE"

# Delete the no longer necessary bootstrap cluster
echo "Rancher cluster has been provisioned and moved. The bootstrap-capi-management cluster can now be deleted."
