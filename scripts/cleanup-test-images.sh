#!/bin/bash

set -e

if [ "$#" -ne 4 ]; then
  echo "Usage: $0 <org> <repo> <package-name> <github-token>"
  exit 1
fi

ORG="$1"
REPO="$2"
PACKAGE_NAME="$3"
GITHUB_TOKEN="$4"

API_URL="https://api.github.com/orgs/$ORG/packages/container/$PACKAGE_NAME/versions?per_page=100"

check_error() {
  local response="$1"
  local context="$2"
  if echo "$response" | jq -e '.message? and .status?' >/dev/null; then
    local error_message
    local error_status
    error_message=$(echo "$response" | jq -r '.message')
    error_status=$(echo "$response" | jq -r '.status')
    echo "Error: $context. $error_message (Status: $error_status)"
    exit 1
  fi
}

if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  CURRENT_DATE=$(date -u -d "1 day ago" +"%Y-%m-%dT%H:%M:%SZ")
elif [[ "$OSTYPE" == "darwin"* ]]; then
  CURRENT_DATE=$(date -u -v-1d +"%Y-%m-%dT%H:%M:%SZ")
else
  echo "Unsupported operating system: $OSTYPE"
  exit 1
fi

response=$(curl -s -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer $GITHUB_TOKEN" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  "$API_URL")

check_error "$response" "Failed to fetch package versions"

echo "$response" | jq -r \
  --arg current_date "$CURRENT_DATE" \
  'map(select(.created_at < $current_date)) | .[] | .id' | while read -r version_id; do
    echo "Deleting version: $version_id"
    delete_response=$(curl -s -X DELETE \
      -H "Authorization: Bearer $GITHUB_TOKEN" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "https://api.github.com/orgs/$ORG/packages/container/$PACKAGE_NAME/versions/$version_id")
    check_error "$delete_response" "Failed to delete version $version_id"
    echo "Successfully deleted version: $version_id"
done

echo "Deletion process complete."
