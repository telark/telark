{{- define "role" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $rules := .rules -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-role
  namespace: {{ $values.app.namespace }}
rules:
{{- range $rule := $rules }}
  - apiGroups: {{ $rule.apiGroups | toJson }}
    resources: {{ $rule.resources | toJson }}
    verbs: {{ $rule.verbs | toJson }}
{{- end }}
{{- end -}}

{{- define "roleBinding" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-role-binding
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
subjects:
  - kind: ServiceAccount
    name: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
    namespace: {{ $values.app.namespace }}
roleRef:
  kind: Role
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}-role
  apiGroup: rbac.authorization.k8s.io
{{- end -}}
