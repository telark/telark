{{- define "telark-core-chart.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
app.kubernetes.io/name: {{ .Chart.Name | quote }}
{{- end }}
