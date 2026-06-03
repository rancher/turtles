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

set -euo pipefail

# Check required tools
for TOOL in kubectl jq; do
    if ! command -v "$TOOL" >/dev/null 2>&1; then
        echo "Error: required tool '$TOOL' not found in PATH." >&2
        exit 1
    fi
done

# Configuration
DRY_RUN=${DRY_RUN:-true}
PHASE=${1:-"pre"} # "pre" or "post"
TURTLES_NS="cattle-turtles-system"
CAAPF_NS="fleet-addon-system"

# Labels
MIGRATION_LABEL="migration.fleet.cattle.io/upgrade-2.14=true"
CAAPF_MANAGED_LABEL="migration.fleet.cattle.io/caapf-managed=true"
PAUSE_LABEL="migration.fleet.cattle.io/paused=true"
MIGRATED_CG_LABEL="migration.fleet.cattle.io/from-caapf=true"
KEEP_RESOURCES_LABEL="migration.fleet.cattle.io/keep-resources=true"

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

# Emit unique "bundle-namespace/bundle-name" keys for BundleDeployments in a namespace,
# excluding the per-cluster fleet-agent system bundle.
bd_bundle_keys() {
    kubectl get bundledeployments -n "$1" -o json 2>/dev/null | jq -r --arg agent "fleet-agent-$2" '
        .items[]
        | select(.metadata.labels["fleet.cattle.io/bundle-namespace"] != null
                 and .metadata.labels["fleet.cattle.io/bundle-name"] != null
                 and .metadata.labels["fleet.cattle.io/bundle-name"] != $agent)
        | "\(.metadata.labels["fleet.cattle.io/bundle-namespace"])/\(.metadata.labels["fleet.cattle.io/bundle-name"])"
    ' | sort -u
}

# Discover all CAAPF-managed clusters by owner references and finalizers.
discover_caapf_clusters() {
    log "Discovering CAAPF-managed clusters..."
    # Get all Fleet clusters that are owned by CAPI clusters.
    local CAPI_OWNED=$(kubectl get clusters.fleet.cattle.io -A -o json | jq -r '
        .items[] | . as $item |
        .metadata.ownerReferences[]? | select(.kind == "Cluster" and (.apiVersion | startswith("cluster.x-k8s.io"))) |
        "\($item.metadata.namespace) \($item.metadata.name) \(.name)"
    ' || true)

    # Iterate over all CAPI-owned Fleet clusters and check if the CAAPF finalizer is present.
    CAAPF_CLUSTERS=()
    while read -r NS FLEET_NAME CAPI_NAME; do
      [ -z "$NS" ] && continue
      FINALIZERS=$(kubectl get clusters.cluster.x-k8s.io "$CAPI_NAME" -n "$NS" -o jsonpath='{.metadata.finalizers}' 2>/dev/null || true)
      if echo "$FINALIZERS" | grep -qF "$CAAPF_FINALIZER"; then
        CAAPF_CLUSTERS+=("$NS/$FLEET_NAME/$CAPI_NAME")
      fi
    done <<< "$CAPI_OWNED"
}

if [ "$PHASE" == "pre" ]; then
    log "Starting pre-upgrade phase for Rancher 2.14..."

    log "Scaling down controllers..."
    for DEPLOY in "rancher-turtles-controller-manager:$TURTLES_NS" "caapf-controller-manager:$CAAPF_NS"; do
        NAME=$(echo "$DEPLOY" | cut -d':' -f1)
        NS=$(echo "$DEPLOY" | cut -d':' -f2)
        if kubectl get deployment -n "$NS" "$NAME" &>/dev/null; then
            run_cmd kubectl scale deployment -n "$NS" "$NAME" --replicas=0
        fi
    done

    # Wait for controller pods to terminate.
    for DEPLOY in "rancher-turtles-controller-manager:$TURTLES_NS" "caapf-controller-manager:$CAAPF_NS"; do
        NAME=$(echo "$DEPLOY" | cut -d':' -f1)
        NS=$(echo "$DEPLOY" | cut -d':' -f2)
        if [ "$DRY_RUN" = "true" ]; then continue; fi
        SELECTOR=$(kubectl get deployment -n "$NS" "$NAME" -o json 2>/dev/null \
            | jq -r '.spec.selector.matchLabels | to_entries | map(.key + "=" + .value) | join(",")' || true)
        if [ -z "$SELECTOR" ] || [ "$SELECTOR" = "null" ]; then
            log "Warning: could not determine pod selector for $NS/$NAME; skipping termination wait."
            continue
        fi
        log "Waiting for $NS/$NAME pods to terminate..."
        if ! kubectl wait --for=delete pod -n "$NS" -l "$SELECTOR" --timeout=120s &>/dev/null; then
            log "Warning: timeout waiting for $NS/$NAME pods to terminate; proceeding anyway."
        fi
    done

    discover_caapf_clusters

    # Map of CAPI namespaces to management clusters.
    declare -A CAPI_NS_TO_MGT
    # Map of ClusterClass namespaces to management clusters.
    declare -A CLASS_NS_TO_MGT
    # Set of CAPI cluster namespaces.
    declare -A CAPI_CLUSTER_NAMESPACES
    # Set of ClusterClass namespaces that are referenced from at least 1 CAPI cluster.
    declare -A CLASS_NAMESPACES

    # Populate maps and sets
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
        CAPI_NAME=$(echo "$ITEM" | cut -d'/' -f3)

        CAPI_NS="$OLD_NS"
        CAPI_CLUSTER_NAMESPACES["$CAPI_NS"]=1

        # Detect cross-namespace ClusterClass reference.
        # v1beta1 uses spec.topology.classNamespace; v1beta2 uses spec.topology.classRef.namespace.
        CLASS_NS=$(kubectl get clusters.cluster.x-k8s.io "$CAPI_NAME" -n "$OLD_NS" -o json 2>/dev/null | jq -r '
                .spec.topology.classRef.namespace //
                .spec.topology.classNamespace //
                empty
            ' || true)
        if [ -n "$CLASS_NS" ] && [ "$CLASS_NS" != "$CAPI_NS" ]; then
            CLASS_NAMESPACES["$CLASS_NS"]=1
        fi

        # Find the management cluster.
        MGT_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

        if [ -n "$MGT_CLUSTER_NAME" ]; then
            existing="${CAPI_NS_TO_MGT[$CAPI_NS]:-}"
            CAPI_NS_TO_MGT["$CAPI_NS"]="${existing:+$existing }$MGT_CLUSTER_NAME"
            if [ -n "$CLASS_NS" ] && [ "$CLASS_NS" != "$CAPI_NS" ]; then
                existing="${CLASS_NS_TO_MGT[$CLASS_NS]:-}"
                CLASS_NS_TO_MGT["$CLASS_NS"]="${existing:+$existing }$MGT_CLUSTER_NAME"
            fi
        fi
    done

    # Copy labels from old Fleet clusters onto their management clusters before the
    # collision check, so that the label-selector query sees the post-migration
    # label set.
    log "Copying Fleet-cluster labels onto management clusters..."
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
        CAPI_NAME=$(echo "$ITEM" | cut -d'/' -f3)
        CAPI_NS="$OLD_NS"

        MGT_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
        [ -z "$MGT_CLUSTER_NAME" ] && continue

        log "Copy labels from $OLD_NAME to $MGT_CLUSTER_NAME..."
        # - KEEP: CAPI/CAAPF labels (cluster.x-k8s.io, fleet.addons.cluster.x-k8s.io)
        # - KEEP: app.kubernetes.io/* recommended labels (user-set application metadata,
        #         e.g. app.kubernetes.io/name; the broad kubernetes.io DROP rule would
        #         otherwise strip these alongside the internal kubernetes.io/* labels)
        # - KEEP: Any custom user labels
        # - DROP: Internal system labels (cattle.io, k8s.io, kubernetes.io)
        LABELS_JSON=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o json 2>/dev/null | jq -c '
            .metadata.labels
                | with_entries(
                    select(
                        (.key | test("cluster\\.x-k8s\\.io|fleet\\.addons\\.cluster\\.x-k8s\\.io|app\\.kubernetes\\.io"))
                    or (
                        (.key | test("cattle\\.io|k8s\\.io|kubernetes\\.io") | not)
                    )
                )
            )' || echo "{}")

        if [ -n "$LABELS_JSON" ] && [ "$LABELS_JSON" != "{}" ]; then
            run_cmd kubectl patch clusters.management.cattle.io "$MGT_CLUSTER_NAME" --type='merge' -p "{\"metadata\":{\"labels\":$LABELS_JSON}}"
        fi
    done

    COLLISION_FOUND=false
    # Map of bundle names to namespaces. 
    declare -A SEEN_BUNDLE_NS
    # Union set of cluster namespaces and clusterclass namespaces. 
    declare -A COLLISION_NAMESPACES
    declare -A COLLISION_NS_TO_MGT

    # Applying BundleNamespaceMappings makes every migration-labeled bundle from every CAPI
    # cluster namespace and class namespace eligible to be deployed to any cluster in fleet-default.
    # Two types of collisions may occur:
    #
    #   1. Name collision: two bundles with the same name from different namespaces both
    #      target the same cluster, producing conflicting BundleDeployments.
    #
    #   2. Label collision: a clusterSelector that was safe under namespace isolation (only
    #      one cluster existed in the namespace) now matches multiple clusters in fleet-default
    #      that happen to share the same labels.
    for NS in "${!CAPI_CLUSTER_NAMESPACES[@]}"; do
        COLLISION_NAMESPACES["$NS"]=1
        COLLISION_NS_TO_MGT["$NS"]="${CAPI_NS_TO_MGT[$NS]:-}"
    done
    for NS in "${!CLASS_NAMESPACES[@]}"; do
        COLLISION_NAMESPACES["$NS"]=1
        existing="${COLLISION_NS_TO_MGT[$NS]:-}"
        COLLISION_NS_TO_MGT["$NS"]="${existing:+$existing }${CLASS_NS_TO_MGT[$NS]:-}"
    done

    log "Checking for bundle collisions before applying BundleNamespaceMappings..."
    for CAPI_NS in "${!COLLISION_NAMESPACES[@]}"; do
        # Check 1: duplicate names across CAPI namespaces.
        RNAMES=$(
            kubectl get bundles.fleet.cattle.io -n "$CAPI_NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true
            kubectl get gitrepos.fleet.cattle.io -n "$CAPI_NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true
            kubectl get helmops.fleet.cattle.io -n "$CAPI_NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true
        )
        for RNAME in $RNAMES; do
            # Skip fleet-agent-* system bundles.
            if [[ "$RNAME" == fleet-agent-* ]]; then continue; fi

            if [ -n "${SEEN_BUNDLE_NS[$RNAME]+_}" ] && [ "${SEEN_BUNDLE_NS[$RNAME]}" != "$CAPI_NS" ]; then
                log "ERROR: Name collision — '$RNAME' in '${SEEN_BUNDLE_NS[$RNAME]}' and '$CAPI_NS' would produce conflicting BundleDeployments in fleet-default."
                COLLISION_FOUND=true
            else
                SEEN_BUNDLE_NS["$RNAME"]="$CAPI_NS"
            fi
        done

        # Check 2: collisions due to labels.
        # Max number of clusters that a selector in this namespace could match.
        EXPECTED_COUNT=$(echo "${COLLISION_NS_TO_MGT[$CAPI_NS]:-}" | wc -w | tr -d ' ')
        # Select targets from all Bundles, GitRepos, and HelmOps in this namespace
        # and create a tab-separated list of "RESOURCE_NAME LABEL_SELECTOR_1,...,LABEL_SELECTOR_N". 
        SELECTOR_PAIRS=$(
            kubectl get bundles.fleet.cattle.io -n "$CAPI_NS" -o json 2>/dev/null | jq -r '
                    .items[] | . as $r |
                    # Skip system bundles
                    select(.metadata.name | startswith("fleet-agent-") | not) |
                    ($r.spec.targets // [])[] |
                    select(.clusterSelector.matchLabels != null) |
                    [$r.metadata.name,
                     (.clusterSelector.matchLabels | to_entries | map(.key + "=" + .value) | join(","))
                    ] | @tsv' || true
            kubectl get gitrepos.fleet.cattle.io -n "$CAPI_NS" -o json 2>/dev/null | jq -r '
                    .items[] | . as $r |
                    ($r.spec.targets // [])[] |
                    select(.clusterSelector.matchLabels != null) |
                    [$r.metadata.name,
                     (.clusterSelector.matchLabels | to_entries | map(.key + "=" + .value) | join(","))
                    ] | @tsv' || true
            kubectl get helmops.fleet.cattle.io -n "$CAPI_NS" -o json 2>/dev/null | jq -r '
                .items[] | . as $r |
                ($r.spec.targets // [])[] |
                select(.clusterSelector.matchLabels != null) |
                [$r.metadata.name,
                 (.clusterSelector.matchLabels | to_entries | map(.key + "=" + .value) | join(","))
                ] | @tsv' || true
        )

        while IFS=$'\t' read -r RNAME LABEL_SEL; do
            [ -z "$RNAME" ] && continue
            MATCH_COUNT=$(kubectl get clusters.management.cattle.io \
                -l "$LABEL_SEL" --no-headers 2>/dev/null \
                | grep -cv '^local[[:space:]]' || true)
            if [ "$MATCH_COUNT" -gt "$EXPECTED_COUNT" ]; then
                log "ERROR: Label collision — '$RNAME' in '$CAPI_NS' selector '$LABEL_SEL' matches $MATCH_COUNT management clusters (expected $EXPECTED_COUNT)."
                COLLISION_FOUND=true
            fi
        done <<< "$SELECTOR_PAIRS"

        # Check 3: Check targets for global deployment risks (empty selectors).
        TARGET_ERRORS=$(
            kubectl get bundles.fleet.cattle.io,gitrepos.fleet.cattle.io,helmops.fleet.cattle.io -n "$CAPI_NS" -o json 2>/dev/null | jq -r '
                .items[] | . as $r |
                select(.metadata.name | startswith("fleet-agent-") | not) |
                if ($r.spec.targets == null or ($r.spec.targets | length == 0)) then
                    "ERROR: \($r.metadata.namespace)/\($r.metadata.name) has NO targets defined. It will target ALL clusters in fleet-default."
                elif (any($r.spec.targets[]; .clusterSelector == null or .clusterSelector == {} or .clusterSelector.matchLabels == {} or (.clusterSelector.matchLabels == null and (.clusterSelector.matchExpressions == null or .clusterSelector.matchExpressions == [])))) then
                    "ERROR: \($r.metadata.namespace)/\($r.metadata.name) has an empty clusterSelector. It will target ALL clusters in fleet-default."
                else
                    empty
                end'
        )
        if [ -n "$TARGET_ERRORS" ]; then
            log "$TARGET_ERRORS"
            COLLISION_FOUND=true
        fi
    done

    log "Checking CAAPF ClusterGroups for collisions before replicating into fleet-default..."
    declare -A CG_SELECTORS=()
    for CG_NS in "${!COLLISION_NAMESPACES[@]}"; do
        CG_ITEMS=$(kubectl get clustergroups.fleet.cattle.io -n "$CG_NS" -o json 2>/dev/null | jq -c '
            .items[]
            | select((.metadata.ownerReferences // []) | map(
                select((.kind == "Cluster" or .kind == "ClusterClass")
                       and (.apiVersion | startswith("cluster.x-k8s.io")))
              ) | length > 0)
            | {name: .metadata.name, selector: (.spec.selector // {})}
        ' || true)
        while IFS= read -r CG; do
            [ -z "$CG" ] && continue
            CG_NAME=$(echo "$CG" | jq -r '.name')
            CG_SEL=$(echo "$CG" | jq -cS '.selector')
            # Prevent empty selectors that would target all clusters in fleet-default.
            IS_EMPTY=$(echo "$CG_SEL" | jq -r '
                ((.matchLabels // {}) | length) as $ml |
                ((.matchExpressions // []) | length) as $me |
                if $ml == 0 and $me == 0 then "true" else "false" end
            ')
            if [ "$IS_EMPTY" = "true" ]; then
                log "ERROR: ClusterGroup '$CG_NS/$CG_NAME' has an empty selector. Replicating it would match ALL clusters in fleet-default."
                COLLISION_FOUND=true
                continue
            fi
            # Same name across namespaces with different selectors can't merge into one
            # ClusterGroup in fleet-default.
            if [ -n "${CG_SELECTORS[$CG_NAME]+_}" ] && [ "${CG_SELECTORS[$CG_NAME]}" != "$CG_SEL" ]; then
                log "ERROR: ClusterGroup '$CG_NAME' exists in multiple namespaces with different selectors; cannot merge into fleet-default."
                COLLISION_FOUND=true
            else
                CG_SELECTORS["$CG_NAME"]="$CG_SEL"
            fi
        done <<< "$CG_ITEMS"
    done

    # Check for pre-existing ClusterGroups of the same name in fleet-default without the migration label.
    # These are user-created and should not be overwritten.
    for CG_NAME in "${!CG_SELECTORS[@]}"; do
        if kubectl get clustergroups.fleet.cattle.io "$CG_NAME" -n fleet-default &>/dev/null; then
            EXISTING_TAG=$(kubectl get clustergroups.fleet.cattle.io "$CG_NAME" -n fleet-default \
                -o jsonpath='{.metadata.labels.migration\.fleet\.cattle\.io/from-caapf}' 2>/dev/null || true)
            if [ "$EXISTING_TAG" != "true" ]; then
                log "ERROR: ClusterGroup '$CG_NAME' already exists in fleet-default without the migration marker; cannot safely overwrite."
                COLLISION_FOUND=true
            fi
        fi
    done

    if [ "$COLLISION_FOUND" = "true" ]; then
        log "Aborting pre-upgrade: resolve the above collisions before proceeding."
        exit 1
    fi
    log "No collisions detected. Proceeding with migration."

    # Copy CAAPF ClusterGroups into fleet-default.
    if [ "${#CG_SELECTORS[@]}" -gt 0 ]; then
        log "Replicating ${#CG_SELECTORS[@]} CAAPF ClusterGroup(s) into fleet-default..."
        for CG_NAME in "${!CG_SELECTORS[@]}"; do
            SEL="${CG_SELECTORS[$CG_NAME]}"
            CG_JSON=$(cat <<EOF
{
  "apiVersion": "fleet.cattle.io/v1alpha1",
  "kind": "ClusterGroup",
  "metadata": {
    "name": "$CG_NAME",
    "namespace": "fleet-default",
    "labels": { "${MIGRATED_CG_LABEL%=*}": "${MIGRATED_CG_LABEL#*=}" }
  },
  "spec": { "selector": $SEL }
}
EOF
)
            if [ "$DRY_RUN" = "true" ]; then
                echo "[DRY-RUN] Would apply ClusterGroup $CG_NAME in fleet-default:"
                echo "$CG_JSON"
            else
                echo "$CG_JSON" | kubectl apply -f -
            fi
        done
    fi

    # Label Fleet clusters for post-phase discovery
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)

        # Label CAAPF clusters so that they can be retrieved in post-upgrade phase
        run_cmd kubectl label clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" "$CAAPF_MANAGED_LABEL" --overwrite
    done

    # Namespaces where at least one Fleet resource was labeled.
    declare -A BNM_NAMESPACES
    # Union set of CAPI cluster namespaces and ClusterClass namespaces.
    declare -A ALL_MIGRATION_NAMESPACES
    for NS in "${!CAPI_CLUSTER_NAMESPACES[@]}" "${!CLASS_NAMESPACES[@]}"; do
        ALL_MIGRATION_NAMESPACES["$NS"]=1
    done

    # keepResources protects the workloads deployed on downstream clusters from being garbage collected.
    log "Setting keepResources=true on Fleet resources before pausing..."
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    for RES in "${RESOURCES[@]}"; do
        for NS in "${!ALL_MIGRATION_NAMESPACES[@]}"; do
            ITEMS=$(kubectl get "$RES" -n "$NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
            for NAME in $ITEMS; do
                if [[ "$RES" == "bundles.fleet.cattle.io" ]]; then
                    # Skip fleet-agent-* system bundles entirely.
                    if [[ "$NAME" == fleet-agent-* ]]; then continue; fi
                    # Skip bundles derived from a GitRepo or HelmOp: they inherit keepResources from their parent resource.
                    REPO_LABEL=$(kubectl get bundles.fleet.cattle.io "$NAME" -n "$NS" \
                        -o jsonpath='{.metadata.labels.fleet\.cattle\.io/repo-name}' 2>/dev/null || true)
                    if [ -n "$REPO_LABEL" ]; then continue; fi
                    HELMOP_LABEL=$(kubectl get bundles.fleet.cattle.io "$NAME" -n "$NS" \
                        -o jsonpath='{.metadata.labels.fleet\.cattle\.io/fleet-helm-name}' 2>/dev/null || true)
                    if [ -n "$HELMOP_LABEL" ]; then continue; fi
                fi

                # Only set and label keepResources if the resource did not already have it.
                IS_KEPT=$(kubectl get "$RES" "$NAME" -n "$NS" -o jsonpath='{.spec.keepResources}' 2>/dev/null || true)
                if [ "$IS_KEPT" != "true" ]; then
                    log "Setting keepResources=true on $RES $NS/$NAME..."
                    run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' -p '{"spec":{"keepResources":true}}'
                    run_cmd kubectl label "$RES" "$NAME" -n "$NS" "$KEEP_RESOURCES_LABEL" --overwrite
                else
                    log "Skipping keepResources for $NS/$NAME (already set)"
                fi
            done
        done
    done

    # Wait for keepResources to propagate to the BundleDeployments that target the old Fleet clusters.
    if [ "$DRY_RUN" != "true" ]; then
        log "Waiting for keepResources to sync to BundleDeployments..."
        KR_WAIT_RETRIES=0
        while true; do
            PENDING=0
            for ITEM in "${CAAPF_CLUSTERS[@]}"; do
                OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
                OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
                BDNS=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath='{.status.namespace}' 2>/dev/null || true)
                [ -z "$BDNS" ] && continue
                # Count non-agent BundleDeployments that have not yet picked up keepResources=true.
                NOT_SYNCED=$(kubectl get bundledeployments -n "$BDNS" -o json 2>/dev/null \
                    | jq -r --arg agent "fleet-agent-$OLD_NAME" '
                        [ .items[]
                          | select(.metadata.labels["fleet.cattle.io/bundle-name"] != $agent)
                          | select((.spec.options.keepResources // false) != true) ] | length' 2>/dev/null || echo 0)
                PENDING=$((PENDING + NOT_SYNCED))
            done
            if [ "$PENDING" -eq 0 ]; then
                log "keepResources synced to all BundleDeployments."
                break
            fi
            KR_WAIT_RETRIES=$((KR_WAIT_RETRIES + 1))
            if [ "$KR_WAIT_RETRIES" -ge 20 ]; then
                log "ERROR: keepResources did not sync to $PENDING BundleDeployment(s) after 10 minutes."
                log "Aborting pre-upgrade: deleting old clusters now would risk deleting downstream workloads."
                exit 1
            fi
            log "Still waiting: $PENDING BundleDeployment(s) without keepResources=true..."
            sleep 30
        done
    fi

    log "Add migration labels to Fleet resources that are located in CAPI cluster and class namespaces..."
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    for RES in "${RESOURCES[@]}"; do
        log "Pausing and labeling $RES..."
        for NS in "${!ALL_MIGRATION_NAMESPACES[@]}"; do
            ITEMS=$(kubectl get "$RES" -n "$NS" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null || true)
            for NAME in $ITEMS; do
                if [[ "$RES" == "bundles.fleet.cattle.io" ]]; then
                    # Skip fleet-agent-* system bundles entirely.
                    if [[ "$NAME" == fleet-agent-* ]]; then
                        log "Skipping system bundle $NAME in $NS"
                        continue
                    fi
                    # Skip bundles derived from a GitRepo: they are handled
                    # alongside the GitRepo resource itself below.
                    REPO_LABEL=$(kubectl get bundles.fleet.cattle.io "$NAME" -n "$NS" \
                        -o jsonpath='{.metadata.labels.fleet\.cattle\.io/repo-name}' 2>/dev/null || true)
                    if [ -n "$REPO_LABEL" ]; then
                        continue
                    fi
                    # Skip bundles derived from a HelmOp: they are handled 
                    # alongside the HelmOp resource itself below.
                    HELMOP_LABEL=$(kubectl get bundles.fleet.cattle.io "$NAME" -n "$NS" \
                        -o jsonpath='{.metadata.labels.fleet\.cattle\.io/fleet-helm-name}' 2>/dev/null || true)
                    if [ -n "$HELMOP_LABEL" ]; then
                        continue
                    fi
                fi

                BNM_NAMESPACES["$NS"]=1

                # Check if resource is already paused.
                IS_PAUSED=$(kubectl get "$RES" "$NAME" -n "$NS" -o jsonpath='{.spec.paused}' 2>/dev/null || true)

                if [ "$IS_PAUSED" != "true" ]; then
                    log "Marking $RES $NS/$NAME for unpausing in post phase..."
                    run_cmd kubectl label "$RES" "$NAME" -n "$NS" "$PAUSE_LABEL" --overwrite
                    run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' -p '{"spec":{"paused":true}}'
                else
                    log "Skipping adding pause label for $NS/$NAME (already paused)"
                fi

                # Add migration label.
                if [[ "$RES" == "helmops.fleet.cattle.io" ]]; then
                    # Propagate the pause label and the migration label to derived bundles, if the script
                    # paused this HelmOp resource earlier. Otherwise just propagate the migration label.
                    if [ "$IS_PAUSED" != "true" ]; then
                        run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' \
                            -p "{\"spec\":{\"labels\":{\"${MIGRATION_LABEL%=*}\":\"true\",\"${PAUSE_LABEL%=*}\":\"true\"}}}"
                    else
                        run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' \
                            -p "{\"spec\":{\"labels\":{\"${MIGRATION_LABEL%=*}\":\"true\"}}}"
                    fi
                else
                    # GitRepo/Bundle: label the resource itself.
                    run_cmd kubectl label "$RES" "$NAME" -n "$NS" "$MIGRATION_LABEL" --overwrite

                    # For GitRepos, also label and pause derived bundles if they are not already paused.
                    if [[ "$RES" == "gitrepos.fleet.cattle.io" ]]; then
                        BUNDLES=$(kubectl get bundles.fleet.cattle.io -n "$NS" -l "fleet.cattle.io/repo-name=$NAME" -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}')
                        for BUNDLE in $BUNDLES; do
                            IS_B_PAUSED=$(kubectl get bundles.fleet.cattle.io "$BUNDLE" -n "$NS" -o jsonpath='{.spec.paused}' 2>/dev/null || true)
                            if [ "$IS_B_PAUSED" != "true" ]; then
                                run_cmd kubectl label bundles.fleet.cattle.io "$BUNDLE" -n "$NS" "$PAUSE_LABEL" --overwrite
                                run_cmd kubectl patch bundles.fleet.cattle.io "$BUNDLE" -n "$NS" --type='merge' -p '{"spec":{"paused":true}}'
                            fi
                            run_cmd kubectl label bundles.fleet.cattle.io "$BUNDLE" -n "$NS" "$MIGRATION_LABEL" --overwrite
                        done
                    fi
                fi
            done
        done
    done

    # Create BundleNamespaceMappings
    for NS in "${!BNM_NAMESPACES[@]}"; do
        MAPPING_YAML=$(cat <<EOF
apiVersion: fleet.cattle.io/v1alpha1
kind: BundleNamespaceMapping
metadata:
  name: migration-mapping-2-14
  namespace: $NS
bundleSelector:
  matchLabels:
    ${MIGRATION_LABEL%=*}: "${MIGRATION_LABEL#*=}"
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

    # Use labels to find the clusters that were marked in the pre phase.
    # This is necessary because Turtles removes the CAAPF finalizer from CAPI clusters when use-caapf=false.
    CAAPF_CLUSTERS=()
    log "Discovering clusters marked for migration..."
    ITEMS=$(kubectl get clusters.fleet.cattle.io -A -l "$CAAPF_MANAGED_LABEL" -o json | jq -r '
        .items[] |
        (
            [ .metadata.ownerReferences[]?
                | select(.kind == "Cluster" and (.apiVersion | startswith("cluster.x-k8s.io")))
                | .name
            ][0] // .metadata.name
        ) as $capi |
        "\(.metadata.namespace) \(.metadata.name) \($capi)"
    ' || true)

    while read -r NS FLEET_NAME CAPI_NAME; do
        [ -z "$NS" ] && continue
        CAAPF_CLUSTERS+=("$NS/$FLEET_NAME/$CAPI_NAME")
    done <<< "$ITEMS"

    # Map old Fleet clusters to new Fleet clusters and copy templateValues.
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
        CAPI_NAME=$(echo "$ITEM" | cut -d'/' -f3)

        # Skip fleet-local and fleet-default namespaces.
        if [[ "$OLD_NS" == "fleet-local" || "$OLD_NS" == "fleet-default" ]]; then
            continue
        fi

        CAPI_NS="$OLD_NS"

        log "Searching for management cluster associated with CAPI Cluster $CAPI_NS/$CAPI_NAME..."

        # Find the Rancher management cluster that owns this CAPI cluster.
        MGT_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)

        if [ -z "$MGT_CLUSTER_NAME" ]; then
            log "Warning: Could not find management cluster for $CAPI_NS/$CAPI_NAME. Skipping templateValues copy."
            continue
        fi

        # The new Fleet cluster name in fleet-default is the same as the management cluster name.
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

        # Copy templateValues.
        TEMPLATE_VALUES=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o json 2>/dev/null \
            | jq -c '.spec.templateValues // {}' \
            || echo "{}")

        if [ -n "$TEMPLATE_VALUES" ] && [ "$TEMPLATE_VALUES" != "{}" ] && [ "$TEMPLATE_VALUES" != "null" ]; then
            log "Copying templateValues from $OLD_NAME to $NEW_FLEET_CLUSTER_NAME..."
            run_cmd kubectl patch clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default --type='merge' -p "{\"spec\":{\"templateValues\":$TEMPLATE_VALUES}}"
        fi
    done

    # Unpause migrated resources.
    log "Unpausing migrated GitRepos, HelmOps and Bundles..."
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    for RES in "${RESOURCES[@]}"; do
        ITEMS=$(kubectl get "$RES" -A -l "${PAUSE_LABEL%=*}=true" -o json 2>/dev/null \
            | jq -r '.items[] | "\(.metadata.namespace) \(.metadata.name)"' || true)
        while read -r NS NAME; do
            [ -z "$NS" ] && continue
            log "Unpausing $RES: $NS/$NAME"
            run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' -p '{"spec":{"paused":false}}'
            # Remove the pause label after unpausing
            run_cmd kubectl label "$RES" "$NAME" -n "$NS" "${PAUSE_LABEL%=*}"-
            # For HelmOps, also strip pause label from spec.labels.
            if [[ "$RES" == "helmops.fleet.cattle.io" ]]; then
                run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' \
                    -p "{\"spec\":{\"labels\":{\"${PAUSE_LABEL%=*}\":null}}}"
            fi
            if [ "$DRY_RUN" != "true" ]; then sleep 5; fi
        done <<< "$ITEMS"
    done

    log "Cleanup: Deleting old CAAPF-managed Fleet clusters..."
    for ITEM in "${CAAPF_CLUSTERS[@]}"; do
        OLD_NS=$(echo "$ITEM" | cut -d'/' -f1)
        OLD_NAME=$(echo "$ITEM" | cut -d'/' -f2)
        CAPI_NAME=$(echo "$ITEM" | cut -d'/' -f3)

        # Safety check: Do not delete clusters in fleet-local or fleet-default
        if [[ "$OLD_NS" == "fleet-local" || "$OLD_NS" == "fleet-default" ]]; then
            log "Skipping deletion of cluster in $OLD_NS namespace: $OLD_NAME"
            continue
        fi

        CAPI_NS="$OLD_NS"

        NEW_FLEET_CLUSTER_NAME=$(kubectl get clusters.management.cattle.io -l "$RANCHER_CAPI_OWNER=$CAPI_NAME,$RANCHER_CAPI_NS=$CAPI_NS" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
        if [ -z "$NEW_FLEET_CLUSTER_NAME" ]; then
            log "Warning: Could not find new Fleet cluster for $OLD_NS/$OLD_NAME. Skipping deletion."
            continue
        fi

        # Wait for the new Fleet cluster to reconcile before comparing bundles.
        log "Waiting for $NEW_FLEET_CLUSTER_NAME to reconcile before comparing bundles..."
        WAIT_RETRIES=0
        while true; do
            IFS='|' read -r STATUS_NS R D <<< "$(kubectl get clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default -o json 2>/dev/null \
                | jq -r '"\(.status.namespace // "")|\(.status.summary.ready // 0)|\(.status.summary.desiredReady // 0)"' \
                || echo "|0|0")" || true
            if [ -n "$STATUS_NS" ] && [ "$R" = "$D" ]; then break; fi
            if [ "$DRY_RUN" = "true" ]; then break; fi
            WAIT_RETRIES=$((WAIT_RETRIES + 1))
            if [ "$WAIT_RETRIES" -ge 20 ]; then
                log "Warning: $NEW_FLEET_CLUSTER_NAME not reconciled (status.namespace='$STATUS_NS' ready=$R desired=$D) after 10 minutes. Skipping deletion."
                continue 2
            fi
            sleep 30
        done

        OLD_BD_NS=$(kubectl get clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS" -o jsonpath='{.status.namespace}' 2>/dev/null || true)
        NEW_BD_NS=$(kubectl get clusters.fleet.cattle.io "$NEW_FLEET_CLUSTER_NAME" -n fleet-default -o jsonpath='{.status.namespace}' 2>/dev/null || true)
        if [ -z "$OLD_BD_NS" ] || [ -z "$NEW_BD_NS" ]; then
            log "Warning: Could not determine BundleDeployment namespaces for $OLD_NAME or $NEW_FLEET_CLUSTER_NAME. Skipping deletion."
            continue
        fi

        # Check that the bundles that target the old cluster are now targeting the new cluster 
        # too. If a bundle was paused before pre-phase then show it as missing and skip deleting
        # the old cluster.
        OLD_KEYS=$(bd_bundle_keys "$OLD_BD_NS" "$OLD_NAME")
        NEW_KEYS=$(bd_bundle_keys "$NEW_BD_NS" "$NEW_FLEET_CLUSTER_NAME")
        MISSING=$(comm -23 <(echo "$OLD_KEYS") <(echo "$NEW_KEYS"))

        if [ -n "$MISSING" ]; then
            log "Warning: bundles present on $OLD_NAME but missing on $NEW_FLEET_CLUSTER_NAME (skipping deletion):"
            log "$MISSING"
            continue
        fi

        log "All bundles from $OLD_NAME present on $NEW_FLEET_CLUSTER_NAME. Deleting old Fleet cluster: $OLD_NS/$OLD_NAME"
        run_cmd kubectl delete clusters.fleet.cattle.io "$OLD_NAME" -n "$OLD_NS"

        # Also delete CAAPF BNM after the old Fleet cluster is deleted.
        CLASS_NS=$(kubectl get clusters.cluster.x-k8s.io "$CAPI_NAME" -n "$OLD_NS" -o json \
            2>/dev/null | jq -r '
                .spec.topology.classRef.namespace //
                .spec.topology.classNamespace //
                empty
            ' || true)
        if [ -n "$CLASS_NS" ] && [ "$CLASS_NS" != "$OLD_NS" ]; then
            if kubectl get bundlenamespacemappings.fleet.cattle.io "$OLD_NS" -n "$CLASS_NS" &>/dev/null; then
                log "Deleting dead CAAPF BNM $CLASS_NS/$OLD_NS (targeted cluster namespace $OLD_NS)..."
                run_cmd kubectl delete bundlenamespacemappings.fleet.cattle.io "$OLD_NS" -n "$CLASS_NS"
            fi
        fi
    done

    # Remove keepResources from the resources this script has marked previously.
    log "Removing keepResources from migrated Fleet resources in fully-migrated namespaces..."
    KR_PENDING_NS=$(kubectl get clusters.fleet.cattle.io -A -l "$CAAPF_MANAGED_LABEL" \
        -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null \
        | sort -u || true)
    RESOURCES=("gitrepos.fleet.cattle.io" "helmops.fleet.cattle.io" "bundles.fleet.cattle.io")
    for RES in "${RESOURCES[@]}"; do
        ITEMS=$(kubectl get "$RES" -A -l "${KEEP_RESOURCES_LABEL%=*}=true" -o json 2>/dev/null \
            | jq -r '.items[] | "\(.metadata.namespace) \(.metadata.name)"' || true)
        while read -r NS NAME; do
            [ -z "$NS" ] && continue
            if [ -n "$KR_PENDING_NS" ] && echo "$KR_PENDING_NS" | grep -qFx "$NS"; then
                log "Skipping keepResources revert for $RES $NS/$NAME — namespace still has CAAPF-managed Fleet clusters."
                continue
            fi
            log "Removing keepResources from $RES $NS/$NAME"
            run_cmd kubectl patch "$RES" "$NAME" -n "$NS" --type='merge' -p '{"spec":{"keepResources":null}}'
            run_cmd kubectl label "$RES" "$NAME" -n "$NS" "${KEEP_RESOURCES_LABEL%=*}"-
        done <<< "$ITEMS"
    done

    log "Delete CAAPF-created ClusterGroups in CAPI and class namespaces..."
    # Skip any namespaces that still contain CAAPF-managed Fleet clusters that are not yet deleted.
    PENDING_NS=$(kubectl get clusters.fleet.cattle.io -A -l "$CAAPF_MANAGED_LABEL" \
        -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null \
        | sort -u || true)
    CG_TO_DELETE=$(kubectl get clustergroups.fleet.cattle.io -A -o json 2>/dev/null | jq -r '
        .items[]
        | select(.metadata.namespace != "fleet-default" and .metadata.namespace != "fleet-local")
        | select((.metadata.ownerReferences // []) | map(
            select((.kind == "Cluster" or .kind == "ClusterClass")
                   and (.apiVersion | startswith("cluster.x-k8s.io")))
          ) | length > 0)
        | "\(.metadata.namespace) \(.metadata.name)"
    ' || true)
    while read -r NS NAME; do
        [ -z "$NS" ] && continue
        if [ -n "$PENDING_NS" ] && echo "$PENDING_NS" | grep -qFx "$NS"; then
            log "Skipping CAAPF ClusterGroup $NS/$NAME — namespace still has CAAPF-managed Fleet clusters."
            continue
        fi
        log "Deleting CAAPF ClusterGroup $NS/$NAME"
        run_cmd kubectl delete clustergroups.fleet.cattle.io "$NAME" -n "$NS"
    done <<< "$CG_TO_DELETE"

    log "Post-upgrade phase complete."
else
    echo "Error: unknown phase '$PHASE'. Usage: $0 [pre|post]" >&2
    exit 1
fi