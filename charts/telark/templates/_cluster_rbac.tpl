{{- define "serviceAccount" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
{{- end -}}

{{- define "clusterRole" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $rules := .rules -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-cluster-role
rules:
{{- range $rule := $rules }}
  - apiGroups: {{ $rule.apiGroups | toJson }}
    resources: {{ $rule.resources | toJson }}
    verbs: {{ $rule.verbs | toJson }}
{{- end }}
{{- end -}}

{{- define "clusterRoleBinding" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-cluster-role-binding
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
subjects:
  - kind: ServiceAccount
    name: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
    namespace: {{ $values.app.namespace }}
roleRef:
  kind: ClusterRole
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-cluster-role
  apiGroup: rbac.authorization.k8s.io
{{- end -}} 