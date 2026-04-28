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

set -e

# Configuration
DRY_RUN=${DRY_RUN:-true}
PHASE=${1:-"pre"} # "pre" or "post"
TURTLES_NS="cattle-turtles-system"
CAAPF_NS="fleet-addon-system"

# Labels
CAPI_NAME_LABEL="cluster.x-k8s.io/cluster-name"
MIGRATION_LABEL="migration.fleet.cattle.io/upgrade-2.14=true"
CAAPF_FLEET_LABEL="migration.fleet.cattle.io/caapf-managed=true"

# Rancher/Turtles labels on Management Clusters (v3)
RANCHER_CAPI_OWNER="cluster-api.cattle.io/capi-cluster-owner"
RANCHER_CAPI_NS="cluster-api.cattle.io/capi-cluster-owner-ns"

# Finalizers
CAAPF_FINALIZER="fleet.addons.cluster.x-k8s.io"

# Helper for dry-run
run_cmd() {
    if [ "$DRY_RUN" = "true" ]; then
        echo "[DRY-RUN] $*"
    else
        "$@"
    fi
}

log() {
    echo -e "\033[1;34m[INFO]\033[0m $*"
}

if [ "$PHASE" == "pre" ]; then
    log "Starting pre-upgrade phase for Rancher 2.14.1..."

    log "Scaling down controllers..."
    for DEPLOY in "rancher-turtles-controller-manager:$TURTLES_NS" "caapf-controller-manager:$CAAPF_NS"; do
        NAME=$(echo "$DEPLOY" | cut -d':' -f1)
        NS=$(echo "$DEPLOY" | cut -d':' -f2)
        if kubectl get deployment -n "$NS" "$NAME" &>/dev/null; then
            run_cmd kubectl scale deployment -n "$NS" "$NAME" --replicas=0
        fi
    done

    log "Propagating CAAPF targeting labels to clusters.management.cattle.io..."
    # Get all Fleet clusters that are owned by CAPI clusters.
    CAPI_OWNED_FLEET_CLUSTERS=$(kubectl get clusters.fleet.cattle.io -A -o json | jq -r '
        .items[] 
            | select(any(.metadata.ownerReferences[]?; .kind == "Cluster" and (.apiVersion | startswith("cluster.x-k8s.io")))) 
            | [
                .metadata.namespace, 
                .metadata.name, 
                (.metadata.ownerReferences[] | select(.kind == "Cluster" and (.apiVersion | startswith("cluster.x-k8s.io"))) | .name)
              ] 
            | join("/")
    ')

    # For each CAPI cluster, if the `fleet.addons.cluster.x-k8s.io` finalizer exists, add it to the list. 
    CAAPF_CLUSTERS=()
    while IFS='/' read -r NS FLEET_NAME CAPI_NAME; do
      FINALIZERS=$(kubectl get clusters.cluster.x-k8s.io "$CAPI_NAME" -n "$NS" -o jsonpath='{.metadata.finalizers}' 2>/dev/null || true)
      if echo "$FINALIZERS" | grep -qF "$CAAPF_FINALIZER"; then
        CAAPF_CLUSTERS+=("$NS/$FLEET_NAME/$CAPI_NAME")
      fi
    done <<< "$CAPI_OWNED_FLEET_CLUSTERS"

    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
        CAPI_NAME=$(echo "$ITEM" | cut -d'/' -f3)

        CAPI_NS="$OLD_NS"
        if [ -z "$CAPI_NAME" ]; then CAPI_NAME=$OLD_NAME; fi

        # Label CAAPF clusters so that they can be retrieved in post-upgrade phase
        run_cmd kubectl label clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" "$CAAPF_FLEET_LABEL" --overwrite

        # Find the cluster resource
        MGT_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

        if [ -n "$MGT_CLUSTER_NAME" ]; then
            log "Copy labels from $OLD_NAME to $MGT_CLUSTER_NAME..."

            # - KEEP: CAPI/CAAPF labels (cluster.x-k8s.io, fleet.addons.cluster.x-k8s.io)
            # - KEEP: Any custom user labels
            # - DROP: Internal system labels (cattle.io, k8s.io, kubernetes.io)
            LABELS_JSON=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o json | jq -c '
                .metadata.labels 
                    | with_entries(
                        select(
                            (.key | test("cluster\\.x-k8s\\.io|fleet\\.addons\\.cluster\\.x-k8s\\.io"))
                        or (
                            (.key | test("cattle\\.io|k8s\\.io|kubernetes\\.io") | not)
                        )
                    )
                )')

            if [ -n "$LABELS_JSON" ] && [ "$LABELS_JSON" != "{}" ]; then
                run_cmd kubectl patch clusters.management.cattle.io "$MGT_CLUSTER_NAME" --type='merge' -p "{\"metadata\":{\"labels\":$LABELS_JSON}}"
            fi
        fi
    done
    
    # Get list of namespaces of CAPI clusters (that are in the same namespace as their CAAPF Fleet clusters)
    declare -A CAPI_CLUSTER_NAMESPACES
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        # Extract namespace from ITEM to use as key
        CAPI_CLUSTER_NAMESPACES["${ITEM%%/*}"]=1
    done

    log "Add migration labels to Fleet resources that are located in CAPI cluster namespaces..."
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    MIGRATED_NAMESPACES=""
    for RES in "${RESOURCES[@]}"; do
        log "Pausing and labeling $RES..."
        for CAPI_CLUSTER_NS in "${!CAPI_CLUSTER_NAMESPACES[@]}"; do
            # Names of Fleet resources of type $RES in namespace $CAPI_CLUSTER_NS
            ITEMS=$(kubectl get "$RES" -n "$CAPI_CLUSTER_NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
            for NAME in $ITEMS; do
                MIGRATED_NAMESPACES="$MIGRATED_NAMESPACES $CAPI_CLUSTER_NS"
                run_cmd kubectl patch "$RES" "$NAME" -n "$CAPI_CLUSTER_NS" --type='merge' -p '{"spec":{"paused":true}}'
        if [[ "$RES" == "helmops.fleet.cattle.io" ]]; then
                    # HelmOp spec.labels propagates to derived bundles, so the
                    # BundleNamespaceMapping can match them.
                    run_cmd kubectl patch "$RES" "$NAME" -n "$CAPI_CLUSTER_NS" --type=merge -p '{"spec":{"labels":{"migration.fleet.cattle.io/upgrade-2.14":"true"}}}'
                elif [[ "$RES" == "bundles.fleet.cattle.io" ]]; then
                    run_cmd kubectl label "$RES" "$NAME" -n "$CAPI_CLUSTER_NS" "$MIGRATION_LABEL" --overwrite
                else
                    # GitRepo: label it and label its derived bundles directly.
                    run_cmd kubectl label "$RES" "$NAME" -n "$CAPI_CLUSTER_NS" "$MIGRATION_LABEL" --overwrite
                    BUNDLES=$(kubectl get bundles.fleet.cattle.io -n "$CAPI_CLUSTER_NS" -l "fleet.cattle.io/repo-name=$NAME" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
                for BUNDLE in $BUNDLES; do
                        run_cmd kubectl label bundles.fleet.cattle.io "$BUNDLE" -n "$CAPI_CLUSTER_NS" "$MIGRATION_LABEL" --overwrite
                done
            fi
            done
        done
    done

    # Create BundleNamespaceMappings
    UNIQUE_NS=$(echo "$MIGRATED_NAMESPACES" | tr ' ' '\n' | sort -u)
    for NS in $UNIQUE_NS; do
        if [ -z "$NS" ]; then continue; fi
        MAPPING_YAML=$(cat <<EOF
apiVersion: fleet.cattle.io/v1alpha1
kind: BundleNamespaceMapping
metadata:
  name: migration-mapping-2-14
  namespace: $NS
bundleSelector:
  matchLabels:
    migration.fleet.cattle.io/upgrade-2.14: "true"
namespaceSelector:
  matchLabels:
    kubernetes.io/metadata.name: fleet-default
EOF
)
    if [ "$DRY_RUN" = "true" ]; then
            echo "[DRY-RUN] Would apply the following BundleNamespaceMapping in $NS:"
            echo "$MAPPING_YAML"
        else
            echo "$MAPPING_YAML" | kubectl apply -f -
    fi
    done
    log "Pre-upgrade phase complete."
elif [ "$PHASE" == "post" ]; then
    log "Starting post-upgrade phase..."

    # 1. Map old Fleet clusters to new Fleet clusters and copy templateValues
    OLD_FLEET_CLUSTERS=$(kubectl get clusters.fleet.cattle.io -A -l "$CAAPF_FLEET_LABEL" -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}')

    for ITEM in $OLD_FLEET_CLUSTERS; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)

        # Skip fleet-local and fleet-default namespaces
        if [[ "$OLD_NS" == "fleet-local" || "$OLD_NS" == "fleet-default" ]]; then
            continue
        fi

        # Get CAPI Cluster details from the old Fleet cluster labels
        # CAAPF propagates labels from the CAPI cluster to the Fleet cluster
        CAPI_NAME=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath="{.metadata.labels['$CAPI_NAME_LABEL']}")
        CAPI_NS="$OLD_NS"

        if [ -z "$CAPI_NAME" ]; then
            # Fallback to name if label is missing
            CAPI_NAME=$OLD_NAME
        fi

        log "Searching for management cluster associated with CAPI Cluster $CAPI_NS/$CAPI_NAME..."

        # Find the Rancher management cluster that owns this CAPI cluster
        MGT_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

        if [ -z "$MGT_CLUSTER_NAME" ]; then
            log "Warning: Could not find management cluster for $CAPI_NS/$CAPI_NAME. Skipping templateValues copy."
            continue
        fi

        # The new Fleet cluster name in fleet-default is the same as the management cluster name
        NEW_FLEET_CLUSTER_NAME=$MGT_CLUSTER_NAME

        log "Waiting for new Fleet cluster '$NEW_FLEET_CLUSTER_NAME' in fleet-default..."
        WAIT_RETRIES=0
        while ! kubectl get clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default &>/dev/null; do
            if [ "$DRY_RUN" = "true" ]; then break; fi
            WAIT_RETRIES=$((WAIT_RETRIES + 1))
            if [ "$WAIT_RETRIES" -ge 20 ]; then
                log "Warning: Timed out waiting for $NEW_FLEET_CLUSTER_NAME after 10 minutes. Skipping."
                continue 2
            fi
            sleep 30
        done

        # Copy templateValues
        TEMPLATE_VALUES=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath='{.spec.templateValues}' 2>/dev/null || echo "{}")

        if [ -n "$TEMPLATE_VALUES" ] && [ "$TEMPLATE_VALUES" != "{}" ]; then
            log "Copying templateValues from $OLD_NAME to $NEW_FLEET_CLUSTER_NAME..."
            run_cmd kubectl patch clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default --type='merge' -p "{\"spec\":{\"templateValues\":$TEMPLATE_VALUES}}"
        fi
    done

    # 2. Unpause migrated resources
    log "Unpausing migrated GitRepos, HelmOps and Bundles..."
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    for RES in "${RESOURCES[@]}"; do
        if [[ "$RES" == "helmops.fleet.cattle.io" ]]; then
            ITEMS=$(kubectl get "$RES" -A -o json | jq -r '
                .items[] |
                select(.spec.labels["migration.fleet.cattle.io/upgrade-2.14"] == "true") |
                .metadata.namespace + "/" + .metadata.name
            ')
        else
            ITEMS=$(kubectl get "$RES" -A -l "$MIGRATION_LABEL" -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}')
        fi
        for ITEM in $ITEMS; do
            NS=$(echo "$ITEM" | cut -d'/' -f1)
            NAME=$(echo "$ITEM" | cut -d'/' -f2)
            log "Unpausing $RES: $NS/$NAME"
            run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' -p '{"spec":{"paused":false}}'
            if [ "$DRY_RUN" != "true" ]; then sleep 5; fi
        done
    done
    
    log "Cleanup: Deleting old CAAPF-managed Fleet clusters..."
    for ITEM in $OLD_FLEET_CLUSTERS; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)

        # Safety check: Do not delete clusters in fleet-local or fleet-default
        if [[ "$OLD_NS" == "fleet-local" || "$OLD_NS" == "fleet-default" ]]; then
            log "Skipping deletion of cluster in $OLD_NS namespace: $OLD_NAME"
            continue
        fi

        CAPI_NAME=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath="{.metadata.labels['$CAPI_NAME_LABEL']}")
        CAPI_NS="$OLD_NS"
        if [ -z "$CAPI_NAME" ]; then CAPI_NAME=$OLD_NAME; fi

        NEW_FLEET_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
        if [ -z "$NEW_FLEET_CLUSTER_NAME" ]; then
            log "Warning: Could not find new Fleet cluster for $OLD_NS/$OLD_NAME. Skipping deletion."
            continue
        fi

        OLD_BD_NS=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath='{.status.namespace}' 2>/dev/null || true)
        NEW_BD_NS=$(kubectl get clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default -o jsonpath='{.status.namespace}' 2>/dev/null || true)
        if [ -z "$OLD_BD_NS" ] || [ -z "$NEW_BD_NS" ]; then
            log "Warning: Could not determine BundleDeployment namespaces for $OLD_NAME or $NEW_FLEET_CLUSTER_NAME. Skipping deletion."
            continue
        fi

        OLD_BD_COUNT=$(kubectl get bundledeployments -n "$OLD_BD_NS" --no-headers 2>/dev/null | wc -l | tr -d ' ')
        NEW_BD_COUNT=$(kubectl get bundledeployments -n "$NEW_BD_NS" --no-headers 2>/dev/null | wc -l | tr -d ' ')
        if [ "$OLD_BD_COUNT" -ne "$NEW_BD_COUNT" ]; then
            log "Warning: BundleDeployment count mismatch for $OLD_NAME ($OLD_BD_COUNT) vs $NEW_FLEET_CLUSTER_NAME ($NEW_BD_COUNT). Skipping deletion."
            continue
        fi

        log "BundleDeployment counts match ($OLD_BD_COUNT). Deleting old Fleet cluster: $OLD_NS/$OLD_NAME"
        run_cmd kubectl delete clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS"
    done

    log "Post-upgrade phase complete."
else
    echo "Error: unknown phase '$PHASE'. Usage: $0 [pre|post]" >&2
    exit 1
fi