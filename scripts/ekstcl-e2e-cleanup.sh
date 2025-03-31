#!/bin/bash

# This is a small script to cleanup all e2e clusters created by eksctl.

set -e

clusters=$(eksctl get clusters --output json | jq '.[].Name')

while IFS=\= read cluster; do
    # Strip quotes from cluster name
    cluster=$(echo "$cluster" | sed 's/"//g')

    # Delete cluster if e2e leftover
    if [[ $cluster == rancher-turtles-e2e* ]] ;
    then
        echo "Deleting e2e cluster $cluster..."
        eksctl delete cluster --name "$cluster" --wait
    fi
done <<END
$clusters
END
