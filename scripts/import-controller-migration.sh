#!/bin/bash

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

namespaces=()
capi_cluster_suffix="\-capi$"
migrated_annotation="cluster-api.cattle.io/migrated"
cluster_owner_label="cluster-api.cattle.io/capi-cluster-owner"       # cluster name
cluster_owner_ns_label="cluster-api.cattle.io/capi-cluster-owner-ns" # cluster namespace

show_help() {
	echo "Usage: ${0##*/} [-h] [namespace...]"
	echo
	echo "Description of the script and its options."
	echo
	echo "Options:"
	echo "  -h, --help       Display this help and exit"
	echo
	echo "Arguments:"
	echo "  namespace        Namespaces to process: you can provide one or more namespaces"
	echo "                   The script will prompt you for confirmation before applying changes to each namespace."
}

while [[ "$#" -gt 0 ]]; do
	case "$1" in
	-h | --help)
		show_help
		exit 0
		;;
	*)
		# Assume any other argument is a namespace
		namespaces+=("$1")
		shift
		;;
	esac
done

add_labels() {
	namespace=$1
	for cluster in $(kubectl get clusters.provisioning.cattle.io -n $namespace -o json | jq -r --arg annotation "$migrated_annotation" '.items[].metadata|select(.annotations[$annotation]!="true")|.name' | grep $capi_cluster_suffix); do
		v3_cluster_name=$(kubectl get clusters.provisioning.cattle.io -n $namespace $cluster -o jsonpath='{.status.clusterName}')
		cluster_base_name=${cluster%-capi}
		echo "The following labels need to be added to cluster $v3_cluster_name"
		echo -e "\t$cluster_owner_label=$cluster_base_name"
		echo -e "\t$cluster_owner_ns_label=$namespace"
		kubectl label clusters.management.cattle.io $v3_cluster_name $cluster_owner_label=$cluster_base_name &&
			kubectl label clusters.management.cattle.io $v3_cluster_name $cluster_owner_ns_label=$namespace
		if [ $? -ne 0 ]; then
			echo "Failed to add labels to cluster $v3_cluster_name: skipping"
			continue
		fi
		echo "Labels added to cluster $v3_cluster_name, setting annotation $migrated_annotation=true on clusters.provisioning.cattle.io cluster $cluster"
		kubectl annotate clusters.provisioning.cattle.io -n $namespace $cluster $migrated_annotation=true
	done
}

for ns in "${namespaces[@]}"; do
	echo "All Rancher clusters in namespace $ns will be updated to prepare for migrating to the new import controller"
	read -p "Are you sure? " -n 1 -r
	echo
	if [[ $REPLY =~ ^[Yy]$ ]]; then
		echo "Updating Rancher clusters in namespace $ns..."
		add_labels $ns
	else
		echo "Namespace $ns skipped"
		continue
	fi
done
