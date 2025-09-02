#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_ARG=""
EXTRA_ADOPTS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --kubeconfig)
      shift
      KUBECONFIG_ARG="--kubeconfig"; export KUBECONFIG=${1:?missing kubeconfig path}; shift ;;
    --kubeconfig=*)
      KUBECONFIG_ARG="--kubeconfig"; export KUBECONFIG="${1#*=}"; shift ;;
    --adopt)
      shift
      EXTRA_ADOPTS+=("${1:?missing name:namespace after --adopt}"); shift ;;
    --adopt=*)
      EXTRA_ADOPTS+=("${1#*=}"); shift ;;
    --help|-h)
      cat <<EOF
Usage: $0 [--kubeconfig PATH] [--adopt name:namespace ...]

Adopts existing resources (namespaces and CAPIProviders) into the Helm release metadata.
Defaults: adopts fleet, rke2-bootstrap, rke2-control-plane in their standard namespaces.

Options:
  --kubeconfig PATH         Use kubeconfig at PATH (also respects KUBECONFIG env)
  --adopt name:namespace    Additionally adopt a CAPIProvider by name and namespace (repeatable)
  -h, --help                Show this help
EOF
      exit 0 ;;
    *)
      echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
done

RELEASE_NAME=${RELEASE_NAME:-rancher-turtles-providers}
RELEASE_NAMESPACE=${RELEASE_NAMESPACE:-rancher-turtles-system}

NAMESPACE_FLEET=${NAMESPACE_FLEET:-rancher-turtles-system}
NAMESPACE_RKE2_BOOTSTRAP=${NAMESPACE_RKE2_BOOTSTRAP:-rke2-bootstrap-system}
NAMESPACE_RKE2_CONTROLPLANE=${NAMESPACE_RKE2_CONTROLPLANE:-rke2-control-plane-system}

echo "Adopting existing turtles resources into Helm ownership"
echo "Release: ${RELEASE_NAME} Namespace: ${RELEASE_NAMESPACE}"

patch_metadata() {
  local kind=$1 name=$2 namespace=${3:-}
  local namespaceFlag=()
  [[ -n $namespace ]] && namespaceFlag=( -n "$namespace" )
  kubectl ${KUBECONFIG_ARG:+$KUBECONFIG_ARG} patch "$kind" "$name" "${namespaceFlag[@]}" \
    --type=merge \
    -p "{\"metadata\":{\"labels\":{\"app.kubernetes.io/managed-by\":\"Helm\"},\"annotations\":{\"meta.helm.sh/release-name\":\"$RELEASE_NAME\",\"meta.helm.sh/release-namespace\":\"$RELEASE_NAMESPACE\"}}}"
}

adopt_namespace() {
  local namespace=$1
  if kubectl ${KUBECONFIG_ARG:+$KUBECONFIG_ARG} get namespace "$namespace" >/dev/null 2>&1; then
    echo "Adopting namespace $namespace"
    patch_metadata namespace "$namespace"
  else
    echo "Namespace $namespace not found, skipping"
  fi
}

adopt_capiprovider() {
  local name=$1 namespace=$2
  if kubectl ${KUBECONFIG_ARG:+$KUBECONFIG_ARG} get capiprovider.turtles-capi.cattle.io "$name" -n "$namespace" >/dev/null 2>&1; then
    echo "Adopting CAPIProvider $name in $namespace"
    patch_metadata capiprovider "$name" "$namespace"
  else
    echo "CAPIProvider $name in $namespace not found, skipping"
  fi
}

for name in fleet rke2-bootstrap rke2-control-plane; do
  namespace=""
  case "$name" in
    fleet) namespace="$NAMESPACE_FLEET" ;;
    rke2-bootstrap) namespace="$NAMESPACE_RKE2_BOOTSTRAP" ;;
    rke2-control-plane) namespace="$NAMESPACE_RKE2_CONTROLPLANE" ;;
  esac
  [[ -z "$namespace" ]] && continue

  adopt_namespace "$namespace"
  adopt_capiprovider "$name" "$namespace"
done

for item in "${EXTRA_ADOPTS[@]}"; do
  name="${item%%:*}"
  namespace="${item#*:}"
  if [[ -z "$name" || -z "$namespace" || "$name" == "$namespace" ]]; then
    echo "Skipping invalid --adopt entry: '$item' (expected name:namespace)" >&2
    continue
  fi
  adopt_namespace "$namespace"
  adopt_capiprovider "$name" "$namespace"
done

echo "Migration completed"
