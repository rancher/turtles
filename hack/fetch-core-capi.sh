#!/bin/bash

# script-specific variables
CAPI_VERSION="${CAPI_VERSION:-latest}"
CAPI_RELEASE_URL="${CAPI_RELEASE_URL:-https://github.com/rancher-sandbox/cluster-api/releases/${CAPI_VERSION}/core-components.yaml}"
CORE_CAPI_NAMESPACE="${CORE_CAPI_NAMESPACE:-cattle-capi-system}"
OUTPUT_FILE="${OUTPUT_FILE:-/tmp/core-provider-configmap.yaml}"

# parameters that must be substituted in CAPI manifest
export CAPI_DIAGNOSTICS_ADDRESS=${CAPI_DIAGNOSTICS_ADDRESS:=:8443}
export CAPI_INSECURE_DIAGNOSTICS=${CAPI_INSECURE_DIAGNOSTICS:=false}
export EXP_MACHINE_POOL=${EXP_MACHINE_POOL:=true}
export EXP_CLUSTER_RESOURCE_SET=${EXP_CLUSTER_RESOURCE_SET:=true}
export CLUSTER_TOPOLOGY=${CLUSTER_TOPOLOGY:=true}
export EXP_RUNTIME_SDK=${EXP_RUNTIME_SDK:=false}
export EXP_MACHINE_SET_PREFLIGHT_CHECKS=${EXP_MACHINE_SET_PREFLIGHT_CHECKS:=true}
export EXP_MACHINE_WAITFORVOLUMEDETACH_CONSIDER_VOLUMEATTACHMENTS=${EXP_MACHINE_WAITFORVOLUMEDETACH_CONSIDER_VOLUMEATTACHMENTS:=true}
export EXP_PRIORITY_QUEUE=${EXP_PRIORITY_QUEUE:=false}

# use CAPI Operator plugin to generate ConfigMap with core CAPI components
kubectl operator preload --core cluster-api --target-namespace ${CORE_CAPI_NAMESPACE} -u ${CAPI_RELEASE_URL} > ${OUTPUT_FILE}
# replace cluster-api-operator managed label with turtles
yq -i 'del(.metadata.labels["managed-by.operator.cluster.x-k8s.io"])'  ${OUTPUT_FILE}
yq -i '.metadata.labels["managed-by.turtles.cattle.io"]="true"' ${OUTPUT_FILE}
