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

set -o errexit
set -o nounset
set -o pipefail

CHART_DIR=${1:?"Usage: $0 <chart dir>"}
CHART_FILE="${CHART_DIR}/Chart.yaml"
CONFIGMAP="${CHART_DIR}/templates/core-provider-configmap.yaml"
export PLACEHOLDER_TAG="v0.0.0"

set_tag() {
    local name=$1 tag=$2

    if [ -z "${tag}" ] || [ "${tag}" = "null" ]; then
        echo "Invalid tag '${tag}' for image ${name}"
        exit 1
    fi

    if [ "$(NAME=${name} yq '.annotations["helm.sh/images"] | from_yaml | .[] | select(.name == strenv(NAME)) | .image | test(":" + strenv(PLACEHOLDER_TAG) + "$")' "${CHART_FILE}")" != "true" ]; then
        echo "Image ${name} with tag ${PLACEHOLDER_TAG} not found in the helm.sh/images annotation of ${CHART_FILE}"
        exit 1
    fi

    NAME=${name} TAG=${tag} yq -i '.annotations["helm.sh/images"] |= (from_yaml
            | (.[] | select(.name == strenv(NAME)) | .image) |= sub(":" + strenv(PLACEHOLDER_TAG) + "$"; ":" + strenv(TAG))
            | to_yaml)' "${CHART_FILE}"
}

set_tag turtles "$(yq '.image.tag' "${CHART_DIR}/values.yaml")"
set_tag cluster-api-controller "$(yq '.metadata.labels["provider.cluster.x-k8s.io/version"]' "${CONFIGMAP}")"
