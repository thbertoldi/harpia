{{- define "odoo.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "odoo.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "odoo.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name (include "odoo.name" .) | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "odoo.namespace" -}}
{{- .Values.namespace.name | default "odoo" }}
{{- end }}

{{- define "odoo.selectorLabels" -}}
app.kubernetes.io/name: {{ include "odoo.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "odoo.labels" -}}
helm.sh/chart: {{ include "odoo.chart" . }}
{{ include "odoo.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "odoo.serviceName" -}}
{{- printf "%s-%s" (include "odoo.fullname" .root) .name | trunc 63 | trimSuffix "-" }}
{{- end }}
