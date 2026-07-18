{{- define "servicePDB" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $pdb := $serviceConfig.pdb | default dict -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $replicas := default $serviceDefaults.replicas $serviceConfig.replicas | default 1 -}}
{{- if $pdb.enabled -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-pdb
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
spec:
  selector:
    matchLabels:
      {{- include "telark.selectorLabels" (dict "root" $root "component" $serviceConfig.name) | nindent 6 }}
{{- if hasKey $pdb "minAvailable" }}
  minAvailable: {{ $pdb.minAvailable }}
{{- else if hasKey $pdb "maxUnavailable" }}
  maxUnavailable: {{ $pdb.maxUnavailable }}
{{- else if le (int $replicas) 1 }}
  maxUnavailable: 0
{{- else }}
  minAvailable: 1
{{- end }}
{{- end -}}
{{- end -}}
