#!/bin/bash

# Check if a filename is provided as an argument
if [ $# -eq 0 ]; then
    echo "Usage: $0 <filename>"
    exit 1
fi

filename=$1

# Define the content to append at the beginning and end of the file
start_content="{{- if index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"enabled\" }}"
end_content="{{- end }}"

# Append content at the beginning of the file
echo "$start_content" | cat - "$filename" > temp && mv temp "$filename"

# Append content at the end of the file
echo "$end_content" >> "$filename"

sed -i '' "s/rancher-turtles-system/{{ index .Values \"rancherTurtles\" \"namespace\" }}/g" "$filename"
sed -i '' "s/rancher-turtles-system/{{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"image\" }}/g" "$filename"
sed -i '' "/image: ghcr.io\/rancher\/turtles-etcd-snapshot-restore:dev/c\\
        {{- \$imageVersion := index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"imageVersion\" -}}\\
        {{- if contains \"sha256:\" \$imageVersion }}\\
        image: {{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"image\" }}@{{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"imageVersion\" }}\\
        {{- else }}\\
        image: {{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"image\" }}:{{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"imageVersion\" }}\\
        {{- end }}\\
" "$filename"
sed -i '' "s/imagePullPolicy: IfNotPresent/imagePullPolicy: '{{ index .Values \"rancherTurtles\" \"features\" \"etcd-snapshot-restore\" \"imagePullPolicy\" }}'/g" "$filename"
