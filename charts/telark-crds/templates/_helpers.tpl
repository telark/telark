{{/*
Base name. app.name (from the telark chart values) is the source of truth for the
app identity. Override with .Values.nameOverride.
*/}}
{{- define "telark.name" -}}
{{- default .Values.app.name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Chart name and version, for the helm.sh/chart label.
*/}}
{{- define "telark.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Recommended labels. Call with the ROOT context:
  {{- include "telark.labels" $ | nindent 4 }}
Per-resource component labels are added at the call site.
*/}}
{{- define "telark.labels" -}}
helm.sh/chart: {{ include "telark.chart" . }}
{{ include "telark.selectorLabels" (dict "root" .) }}
app.kubernetes.io/part-of: {{ include "telark.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Identity label subset. Call with a dict:
  {{- include "telark.selectorLabels" (dict "root" $ "component" "crd") }}
*/}}
{{- define "telark.selectorLabels" -}}
app.kubernetes.io/name: {{ include "telark.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- with .component }}
app.kubernetes.io/component: {{ . }}
{{- end }}
{{- end -}}
