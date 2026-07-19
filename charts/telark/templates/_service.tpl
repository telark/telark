{{- define "serviceService" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $port := default $serviceDefaults.port $serviceConfig.port | default 8080 -}}
{{- $serviceType := default $serviceDefaults.serviceType $serviceConfig.serviceType | default "ClusterIP" -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
  {{- with (include "telark.annotations" $root) }}
  annotations:
    {{- . | nindent 4 }}
  {{- end }}
spec:
  selector:
    {{- include "telark.selectorLabels" (dict "root" $root "component" $serviceConfig.name) | nindent 4 }}
  ports:
    - name: http
      port: {{ $port }}
      protocol: TCP
      targetPort: {{ $port }}
  type: {{ $serviceType }}
{{- end -}}
