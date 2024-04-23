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

