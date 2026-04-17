#!/usr/bin/env bash

# Copyright © 2023 - 2026 SUSE LLC
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

# Cleanup Fleet clusters created by CAAPF
# Iterate over CAPI cluster objects in all namespaces
# For each CAPI cluster, get Fleet clusters in the same namespace
# If name of Fleet cluster and CAPI cluster are the same, and Fleet cluster is labeled with `cluster.x-k8s.io/cluster-name: <name of capi cluster>`, delete the Fleet cluster object

set -e

# Change this to 'false' only when you're ready to actually delete resources
DRY_RUN=true

echo "======================================================"
echo "Cleaning up CAAPF Fleet Clusters (Dry Run: $DRY_RUN)"
echo "======================================================"

# Iterate over CAPI clusters in all namespaces
kubectl get clusters.cluster.x-k8s.io -A -o custom-columns="NS:.metadata.namespace,NAME:.metadata.name" --no-headers | while read -r namespace capi_name; do

    # Skip empty lines just in case
    if [[ -z "$namespace" || -z "$capi_name" ]]; then
        continue
    fi

    # Look for the Fleet cluster in the exact same namespace, with the exact same name
    # and the specific label
    target=$(kubectl get clusters.fleet.cattle.io \
        -n "$namespace" \
        --field-selector "metadata.name=$capi_name" \
        -l "cluster.x-k8s.io/cluster-name=$capi_name" \
        -o name \
        --ignore-not-found)

    # Delete if DRY_RUN is disabled
    if [[ -n "$target" ]]; then
        if [ "$DRY_RUN" = true ]; then
            echo "[DRY RUN] Would delete Fleet cluster '$capi_name' in namespace '$namespace'"
        else
            echo "[ACTION] Deleting Fleet cluster '$capi_name' in namespace '$namespace'..."
            kubectl delete clusters.fleet.cattle.io "$capi_name" -n "$namespace"
        fi
    fi

    # Check for and remove namespace annotation
    if kubectl get namespace "$namespace" -o jsonpath='{.metadata.annotations}' | grep -q "field.cattle.io/allow-fleetworkspace-creation-for-existing-namespace"; then

        if [ "$DRY_RUN" = true ]; then
            echo "[DRY RUN] Would remove annotation 'field.cattle.io/allow-fleetworkspace-creation-for-existing-namespace' from namespace '$namespace'"
        else
            echo "[ACTION] Removing annotation from namespace '$namespace'..."
            kubectl annotate namespace "$namespace" field.cattle.io/allow-fleetworkspace-creation-for-existing-namespace-
        fi

    fi

done

echo "Fleet cluster cleanup done."

echo "======================================================"
echo "Cleaning up CAAPF ClusterClass-related resources (Dry Run: $DRY_RUN)"
echo "======================================================"

# Iterate over CAPI ClusterClasses (clusterclasses.cluster.x-k8s.io) in all namespaces
# For each ClusterClass, look for:
# - Fleet cluster group: clustergroup.fleet.cattle.io -> these are labeled with `clusterclass-name.fleet.addons.cluster.x-k8s.io=<name of class>` and `clusterclass-namespace.fleet.addons.cluster.x-k8s.io=<namespace of class>`
# - Fleet bundlenamespacemapping: bundlenamespacemapping.fleet.cattle.io -> this exists in the namespace of the clusterclass

# Iterate over CAPI ClusterClasses in all namespaces
kubectl get clusterclasses.cluster.x-k8s.io -A -o custom-columns="NS:.metadata.namespace,NAME:.metadata.name" --no-headers | while read -r cc_namespace cc_name; do

    # Skip empty lines
    if [[ -z "$cc_namespace" || -z "$cc_name" ]]; then
        continue
    fi

    # Find matching Fleet ClusterGroups across all namespaces using the labels
    kubectl get clustergroups.fleet.cattle.io -A \
        -l "clusterclass-name.fleet.addons.cluster.x-k8s.io=$cc_name,clusterclass-namespace.fleet.addons.cluster.x-k8s.io=$cc_namespace" \
        -o custom-columns="CG_NS:.metadata.namespace,CG_NAME:.metadata.name" --no-headers | while read -r cg_namespace cg_name; do

        if [[ -z "$cg_namespace" || -z "$cg_name" ]]; then
            continue
        fi

        if [ "$DRY_RUN" = true ]; then
            echo "[DRY RUN] Would delete Fleet ClusterGroup '$cg_name' in namespace '$cg_namespace'"
        else
            echo "[ACTION] Deleting Fleet ClusterGroup '$cg_name' in namespace '$cg_namespace'..."
            kubectl delete clustergroups.fleet.cattle.io "$cg_name" -n "$cg_namespace"
        fi
    done

    # Find BundleNamespaceMappings in the same namespace as the ClusterClass
    kubectl get bundlenamespacemappings.fleet.cattle.io -n "$cc_namespace" \
        -o custom-columns="BNM_NAME:.metadata.name" --no-headers --ignore-not-found | while read -r bnm_name; do

        if [[ -z "$bnm_name" ]]; then
            continue
        fi

        if [ "$DRY_RUN" = true ]; then
            echo "[DRY RUN] Would delete Fleet BundleNamespaceMapping '$bnm_name' in namespace '$cc_namespace'"
        else
            echo "[ACTION] Deleting Fleet BundleNamespaceMapping '$bnm_name' in namespace '$cc_namespace'..."
            kubectl delete bundlenamespacemappings.fleet.cattle.io "$bnm_name" -n "$cc_namespace"
        fi
    done

done

echo "CAAPF ClusterClass resources cleanup done."
