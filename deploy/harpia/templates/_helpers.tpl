{{- define "harpia.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "harpia.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "harpia.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "harpia.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "harpia.selectorLabels" -}}
app.kubernetes.io/name: {{ include "harpia.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "harpia.labels" -}}
helm.sh/chart: {{ include "harpia.chart" . }}
{{ include "harpia.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "harpia.serviceName" -}}
{{- $name := .name }}
{{- printf "%s-%s" (include "harpia.fullname" .root) $name }}
{{- end }}

{{- define "harpia.image" -}}
{{- $registry := .root.Values.image.registry | default "ghcr.io" }}
{{- $repo := .imageRepo | default .root.Values.image.repository }}
{{- $tag := .imageTag | default .root.Values.image.tag }}
{{- printf "%s/%s:%s" $registry $repo $tag }}
{{- end }}
