{{- define "servicePDB" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $pdb := $serviceConfig.pdb | default dict -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $replicas := default $serviceDefaults.replicas $serviceConfig.replicas | default 1 -}}
{{- if $pdb.enabled -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-pdb
  namespace: {{ $values.app.namespace }}
  labels:
    app.kubernetes.io/name: {{ $values.app.name }}-{{ $serviceConfig.name }}
    app.kubernetes.io/part-of: {{ $values.app.name }}
spec:
  selector:
    matchLabels:
      app: {{ $values.app.name }}-{{ $serviceConfig.name }}
      type: {{ $serviceConfig.category }}
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
