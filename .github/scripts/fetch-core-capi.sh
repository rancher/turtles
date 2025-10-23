#!/bin/bash

# script-specific variables
CAPI_VERSION="${CAPI_VERSION:-latest}"
CAPI_RELEASE_URL="${CAPI_RELEASE_URL:-https://github.com/rancher-sandbox/cluster-api/releases/${CAPI_VERSION}/core-components.yaml}"
CORE_CAPI_NAMESPACE="${CORE_CAPI_NAMESPACE:-cattle-capi-system}"
OUTPUT_DIR="${OUTPUT_DIR:-/tmp}"
OUTPUT_FILE="${OUTPUT_FILE:-core-provider-configmap.yaml}"
MANAGED_BY_LABEL="managed-by.turtles.cattle.io"

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

# install krew and CAPI Operator plugin
set -x
OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
    ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
    KREW="krew-${OS}_${ARCH}" &&
    curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" &&
    tar zxvf "${KREW}.tar.gz" &&
    ./"${KREW}" install krew
export PATH="${KREW_ROOT:-$HOME/.krew}/bin:$PATH"
kubectl krew index add operator https://github.com/kubernetes-sigs/cluster-api-operator.git
kubectl krew install operator/clusterctl-operator
kubectl operator version

# use CAPI Operator plugin to generate ConfigMap with core CAPI components
kubectl operator preload --core cluster-api:${CORE_CAPI_NAMESPACE} -u ${CAPI_RELEASE_URL} >>${OUTPUT_DIR}/${OUTPUT_FILE}
# this is needed to remove comments in the yaml manifest that contain '{{' which breaks Helm parsing
sed -i '/{{[^-]/d' ${OUTPUT_DIR}/${OUTPUT_FILE}
# label as managed by turtles for easier filtering
sed -i -r 's/^(\s*)(provider\.cluster\.x-k8s\.io\/version:.*)/\1\2\n\1'"${MANAGED_BY_LABEL}"': "true"/' ${OUTPUT_DIR}/${OUTPUT_FILE}
# change namespace from capi-system to cattle-capi-system
sed -i 's/: capi-system/: cattle-capi-system/g' ${OUTPUT_DIR}/${OUTPUT_FILE}
sed -i 's/.capi-system.svc/.cattle-capi-system.svc/g' ${OUTPUT_DIR}/${OUTPUT_FILE}

# embed this in Turtles chart
mv ${OUTPUT_DIR}/${OUTPUT_FILE} ./charts/rancher-turtles/templates/${OUTPUT_FILE}
