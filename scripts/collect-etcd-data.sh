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

# This script dumps some etcd info.
# Mainly this is used to detect and debug conflicts managing resources.
 
set -e

# OUTPUT_DIR is the directory where to store results.
OUTPUT_DIR=${OUTPUT_DIR:-./_artifacts/etcd}
# CLUSTER_NAME is a name prefix added to all collected artifacts.
CLUSTER_NAME=${CLUSTER_NAME:-capi-test}
# CONTROL_PLANE_CONTAINER_NAME is the docker container name used to lookup ETCD certificates.
CONTROL_PLANE_CONTAINER_NAME=${CONTROL_PLANE_CONTAINER_NAME:-capi-test-control-plane}
# ETCD_ENDPOINT_ADDRESS is the ETCD listener address
ETCD_ENDPOINT_ADDRESS=${ETCD_ENDPOINT_ADDRESS:-127.0.0.1}
# ETCD_ENDPOINT_PORT is the ETCD listener port
ETCD_ENDPOINT_PORT=${ETCD_ENDPOINT_PORT:-30002}

etcd_endpoint=https://$ETCD_ENDPOINT_ADDRESS:$ETCD_ENDPOINT_PORT
intermediate_dir=/tmp/etcd-collection/$CLUSTER_NAME

mkdir -p $OUTPUT_DIR

# Initialize intermediate directory
rm -rf $intermediate_dir
mkdir -p $intermediate_dir

# Fetch etcd credentials
echo "Fetching etcd credentials from container $CONTROL_PLANE_CONTAINER_NAME"
docker cp $CONTROL_PLANE_CONTAINER_NAME:/etc/kubernetes/pki/etcd/ca.crt $intermediate_dir/ca.crt
docker cp $CONTROL_PLANE_CONTAINER_NAME:/etc/kubernetes/pki/etcd/peer.crt $intermediate_dir/peer.crt
docker cp $CONTROL_PLANE_CONTAINER_NAME:/etc/kubernetes/pki/etcd/peer.key $intermediate_dir/peer.key

# Dump human readable status
echo "Dumping endpoint status: $OUTPUT_DIR/$CLUSTER_NAME-status.txt"
etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --insecure-skip-tls-verify --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key endpoint status --write-out=table > $OUTPUT_DIR/$CLUSTER_NAME-status.txt

# Dump keys collection, sorted by versions
echo "Collecting keys... this will take a while"

# For each key, dump some info
for key in `etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key get --prefix --keys-only /`
do
  size=`etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key get $key --print-value-only | wc -c`
  count=`etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key get $key --write-out=fields | grep \"Count\" | cut -f2 -d':'`
  if [ $count -ne 0 ]; then
    versions=`etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key get $key --write-out=fields | grep \"Version\" | cut -f2 -d':'`
  else
    versions=0
  fi
  total=$(($size * $versions))
  printf "$total\t$size\t$versions\t$count\t$key\n" >> $intermediate_dir/keys.txt
done

# Sort keys by versions count
echo "Dumping sorted keys: $OUTPUT_DIR/$CLUSTER_NAME-keys-sorted.txt"
printf "TOTAL\tSIZE\tVERSIONS\tCOUNT\tKEY\n" > $OUTPUT_DIR/$CLUSTER_NAME-keys-sorted.txt
sort -nrk 3 $intermediate_dir/keys.txt >> $OUTPUT_DIR/$CLUSTER_NAME-keys-sorted.txt

# Exit with error if size exceeded
echo "Taking snapshot to calculate database size"
etcdctl --endpoints=$etcd_endpoint --cacert=$intermediate_dir/ca.crt --cert=$intermediate_dir/peer.crt --key=$intermediate_dir/peer.key snapshot save $intermediate_dir/snapshot.db
size=$(($(stat -c%s $intermediate_dir/snapshot.db) / 1000000))
echo "Calculated size from snapshot is $size MB"

rm -rf $intermediate_dir # Ensure cleanup to immediately remove large snapshots.

size_limit=500
if (( size > size_limit )); then
  printf "Error: ETCD database size exceeding limit of $size_limit MB. Found: $size MB\n" >&2
  exit 1
fi

exit 0
