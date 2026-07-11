{{- define "serviceAccount" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
  namespace: {{ $values.app.namespace }}
  labels:
    app.kubernetes.io/name: {{ $values.app.name }}-{{ $serviceConfig.name }}
    app.kubernetes.io/part-of: {{ $values.app.name }}
{{- end -}}

{{- define "clusterRole" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $rules := .rules -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    app.kubernetes.io/name: {{ $values.app.name }}-{{ $serviceConfig.name }}
    app.kubernetes.io/part-of: {{ $values.app.name }}
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
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-cluster-role-binding
subjects:
  - kind: ServiceAccount
    name: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
    namespace: {{ $values.app.namespace }}
roleRef:
  kind: ClusterRole
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-cluster-role
  apiGroup: rbac.authorization.k8s.io
{{- end -}} 