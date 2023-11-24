{{/*
    This removes the part after the + in the kubernetes version string.
    v1.27.4+k3s1 -> v1.27.4
    v1.28.0 -> v1.28.0
*/}}
{{- define "strippedKubeVersion" -}}
{{- $parts := split "+" .Capabilities.KubeVersion.Version -}}
{{- print $parts._0 -}}
{{- end -}}
