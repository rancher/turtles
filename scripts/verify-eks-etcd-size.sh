#!/usr/bin/env bash

# Copyright © 2026 SUSE LLC
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

# KUBECONFIG_PATH is the path to the kubeconfig for the cluster.
KUBECONFIG_PATH=${KUBECONFIG_PATH:-""}

echo "Calculating etcd size using kubeconfig: $KUBECONFIG_PATH"

size=`kubectl --kubeconfig $KUBECONFIG_PATH get --raw /metrics | grep ^apiserver_storage_size_bytes | awk  '{print $2}' | awk -F"E" 'BEGIN{OFMT="%10.0f"} {print $1 * (10 ^ $2) / 1000000}' | xargs`

echo "Calculated etcd size from metrics is $size MB"

size_limit=500
if (( size > size_limit )); then
  printf "Error: ETCD database size exceeding limit of $size_limit MB. Found: $size MB\n" >&2
  exit 1
fi

exit 0
