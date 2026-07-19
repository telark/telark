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
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-role
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
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-role-binding
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
subjects:
  - kind: ServiceAccount
    name: {{ include "telark.serviceAccountName" (dict "root" $root "serviceConfig" $serviceConfig) }}
    namespace: {{ $values.app.namespace }}
roleRef:
  kind: Role
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-role
  apiGroup: rbac.authorization.k8s.io
{{- end -}}
