{{- define "serviceAccount" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $sa := $serviceConfig.serviceAccount | default dict -}}
{{- if dig "create" true $sa }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "telark.serviceAccountName" (dict "root" $root "serviceConfig" $serviceConfig) }}
  namespace: {{ $values.app.namespace }}
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
  {{- with $sa.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
{{- if hasKey $serviceConfig "automountServiceAccountToken" }}
automountServiceAccountToken: {{ $serviceConfig.automountServiceAccountToken }}
{{- end }}
{{- end }}
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
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-cluster-role
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
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-cluster-role-binding
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
subjects:
  - kind: ServiceAccount
    name: {{ include "telark.serviceAccountName" (dict "root" $root "serviceConfig" $serviceConfig) }}
    namespace: {{ $values.app.namespace }}
roleRef:
  kind: ClusterRole
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-cluster-role
  apiGroup: rbac.authorization.k8s.io
{{- end -}}
