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

# Run your command and capture its output
output=$(make docker-list-all REGISTRY="$1" ORG="$2" TAG="$3")

# Use a for loop to iterate over each line
IFS=$'\n'       # Set the Internal Field Separator to newline
line_count=0    # Counter to keep track of the current line
total_lines=$(echo "$output" | wc -l)  # Get the total number of lines
githubimageoutput=("multiarch_image" "amd64_image" "arm64_image")
githubdigestoutput=("multiarch_digest" "amd64_digest" "arm64_digest")

for line in $output; do
  # Run the Docker command and get the digest
  digest=$(docker buildx imagetools inspect "$line" --format '{{json .}}' | jq -r .manifest.digest)

  # Add encoded image name to the output
  image_output=$(echo -n "$line" | base64 -w0 | base64 -w0)
  echo "${githubimageoutput[$line_count]}=${image_output}" >> "$GITHUB_OUTPUT"
  # Add encoded digest to the output
  digest_output=$(echo -n "$digest" | base64 -w0 | base64 -w0)
  echo "${githubdigestoutput[$line_count]}=${digest_output}" >> "$GITHUB_OUTPUT"

  # Increment the line counter
  line_count=$((line_count + 1))
done
