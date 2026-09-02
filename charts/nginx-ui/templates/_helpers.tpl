{{- define "nginx-ui.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "nginx-ui.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{- define "nginx-ui.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "nginx-ui.labels" -}}
helm.sh/chart: {{ include "nginx-ui.chart" . }}
{{ include "nginx-ui.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}

{{- define "nginx-ui.selectorLabels" -}}
app.kubernetes.io/name: {{ include "nginx-ui.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "nginx-ui.image" -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) }}
{{- end }}

{{- define "nginx-ui.claimName" -}}
{{- $root := index . 0 -}}
{{- $volume := index . 1 -}}
{{- $settings := index $root.Values.persistence $volume -}}
{{- if $settings.existingClaim -}}
{{- $settings.existingClaim -}}
{{- else -}}
{{- printf "%s-%s" (include "nginx-ui.fullname" $root) (kebabcase $volume) -}}
{{- end -}}
{{- end }}
