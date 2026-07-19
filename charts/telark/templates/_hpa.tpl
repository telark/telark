{{- define "serviceHPA" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $autoscaling := merge (deepCopy ($serviceConfig.autoscaling | default dict)) ($serviceDefaults.autoscaling | default dict) -}}
{{- if $autoscaling.enabled -}}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-hpa
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}
  minReplicas: {{ $autoscaling.minReplicas | default 2 }}
  maxReplicas: {{ $autoscaling.maxReplicas | default 5 }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ $autoscaling.targetCPUUtilizationPercentage | default 80 }}
    {{- with $autoscaling.targetMemoryUtilizationPercentage }}
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: {{ . }}
    {{- end }}
{{- end -}}
{{- end -}}
