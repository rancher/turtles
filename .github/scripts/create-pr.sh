#!/bin/bash

target_branch=$1
new_turtles=$2
new_chart=$3
repo=$4

# Check if the environment variable is set
if [ -z "$GITHUB_TOKEN" ]; then
    echo "Environment variable GITHUB_TOKEN is not set."
    exit 1
fi

# Configure git login
git config --local --unset http.https://github.com/.extraheader ^AUTHORIZATION:
gh auth setup-git

# Create and push new branch
git remote add fork "https://github.com/rancherbot/$repo"
BRANCH_NAME="turtles-$(date +%s)"
git checkout -b "$BRANCH_NAME"
git push fork "$BRANCH_NAME"

# Create a pull request
gh pr create --title "[${target_branch}] turtles ${new_chart}+up${new_turtles} update" \
             --body "Update Turtles to v${new_turtles}"$'\n\n'"Changelog: https://github.com/rancher/turtles/releases/tag/v${new_turtles}" \
             --base "${target_branch}" \
             --repo "rancher/$repo" --head "rancherbot:$BRANCH_NAME"