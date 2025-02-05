#!/bin/bash

# Check if a filename is provided as an argument
if [ $# -ne 2 ]; then
    echo "Usage: $0 <feature> <filename>"
    exit 1
fi

feature=$1
filename=$2

# Determine the appropriate sed command
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed_cmd="gsed"
else
    sed_cmd="sed"
fi

# Define the content to append at the beginning and end of the file
start_content="{{- if index .Values \"rancherTurtles\" \"features\" \"$feature\" \"enabled\" }}"
end_content="{{- end }}"

# Append content at the beginning of the file
echo "$start_content" | cat - "$filename" >temp && mv temp "$filename"

# Append content at the end of the file
echo "$end_content" >>"$filename"

# 1. Replace "rancher-turtles-system" with the templated namespace
$sed_cmd -i "s|rancher-turtles-system|{{ index .Values \"rancherTurtles\" \"namespace\" }}|g" "$filename"

# 2. Replace "rancher-turtles-system" with the templated image
$sed_cmd -i "s|rancher-turtles-system|{{ index .Values \"rancherTurtles\" \"features\" \"$feature\" \"image\" }}|g" "$filename"

# 3. Update the "image:" section dynamically based on conditions
$sed_cmd -i '/image: ghcr.io\/rancher\/turtles-'${feature}':dev/c\
        {{- $imageVersion := index .Values "rancherTurtles" "features" "'${feature}'" "imageVersion" -}}\
        {{- if contains "sha256:" $imageVersion }}\
        image: {{ index .Values "rancherTurtles" "features" "'${feature}'" "image" }}@{{ index .Values "rancherTurtles" "features" "'${feature}'" "imageVersion" }}\
        {{- else }}\
        image: {{ index .Values "rancherTurtles" "features" "'${feature}'" "image" }}:{{ index .Values "rancherTurtles" "features" "'${feature}'" "imageVersion" }}\
        {{- end }}' "$filename"

# 4. Replace the "imagePullPolicy" dynamically
$sed_cmd -i "s|imagePullPolicy: IfNotPresent|imagePullPolicy: '{{ index .Values \"rancherTurtles\" \"features\" \"$feature\" \"imagePullPolicy\" }}'|g" "$filename"

# Confirmation message
echo "All replacements applied successfully to $filename"
