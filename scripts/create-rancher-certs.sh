#!/bin/bash

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

# Creates self signed certificates to configure Rancher's ingress.
# Files can be loaded before installing Rancher with:
#   kubectl create namespace cattle-system
#   kubectl -n cattle-system create secret tls tls-rancher-ingress \
#     --cert=$RANCHER_CERT_PATH \
#     --key=$RANCHER_KEY_PATH
#   kubectl -n cattle-system create secret generic tls-ca \
#     --from-file=$RANCHER_CACERT_PATH

set -xe

RANCHER_HOSTNAME=${RANCHER_HOSTNAME:-localhost}
RANCHER_CERT_DIR=${RANCHER_CERT_DIR:-/tmp/rancher-private-ca}
RANCHER_CERT_PATH=${RANCHER_CERT_PATH:-$RANCHER_CERT_DIR/tls.crt}
RANCHER_CERT_KEY_PATH=${RANCHER_CERT_KEY_PATH:-$RANCHER_CERT_DIR/tls.key}
RANCHER_CACERT_PATH=${RANCHER_CACERT_PATH:-$RANCHER_CERT_DIR/cacerts.pem}

mkdir -p $RANCHER_CERT_DIR

# Generate CA cert
openssl genrsa -out "$RANCHER_CERT_DIR/cacerts.key" 4096
openssl req -x509 -new -nodes \
  -key "$RANCHER_CERT_DIR/cacerts.key" \
  -sha256 -days 3650 \
  -out "$RANCHER_CACERT_PATH" \
  -subj "/CN=Rancher Test"

# Generate tls cert
openssl genrsa -out "$RANCHER_CERT_KEY_PATH" 2048
openssl req -new \
  -key "$RANCHER_CERT_KEY_PATH" \
  -out "$RANCHER_CERT_DIR/tls.csr" \
  -subj "/CN=localhost"
cat > $RANCHER_CERT_DIR/tls.v3.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = $RANCHER_HOSTNAME
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF
openssl x509 -req \
  -in "$RANCHER_CERT_DIR/tls.csr" \
  -CA "$RANCHER_CACERT_PATH" \
  -CAkey "$RANCHER_CERT_DIR/cacerts.key" \
  -CAcreateserial \
  -out "$RANCHER_CERT_PATH" \
  -days 3650 \
  -sha256 \
  -extfile "$RANCHER_CERT_DIR/tls.v3.ext"
