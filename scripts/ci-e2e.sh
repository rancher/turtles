#!/bin/bash

################################################################################
# usage: ./ci-e2e.sh
#  This program runs the e2e tests.
#  It is automatically triggered by Rancher Turtles CI system.
#
# If you want to run it locally, you'll need to export the following environment variables:
#    - Ngrok:   NGROK_AUTHTOKEN, NGROK_API_KEY
#    - Rancher: RANCHER_HOSTNAME, RANCHER_PASSWORD
#    - Azure:   AZURE_SUBSCRIPTION_ID, AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET
#    - AWS:     CAPA_ENCODED_CREDS
#
################################################################################

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

# shellcheck source=hack/utils.sh
source "${REPO_ROOT}/hack/utils.sh"

# Verify the required environment variables
turtles::utils::ensure_ngrok_envs
turtles::utils::ensure_rancher_envs
turtles::utils::ensure_azure_envs
turtles::utils::ensure_aws_envs

# Run E2E
make test-e2e

# Run janitors
# Azure janitor
#
# AWS janitor
#
