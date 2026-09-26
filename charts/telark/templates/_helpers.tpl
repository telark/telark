{{/*
Base name. app.name is the single source of truth for the app identity.
Override with .Values.nameOverride.
*/}}
{{- define "telark.name" -}}
{{- default .Values.app.name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource-name prefix for every rendered object. Stable and release-independent by
design: services address each other by these names, so it must not vary with the
release name. Override with .Values.fullnameOverride.
*/}}
{{- define "telark.fullname" -}}
{{- default .Values.app.name .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Chart name and version, for the helm.sh/chart label.
*/}}
{{- define "telark.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Recommended labels applied to every telark resource. Call with the ROOT context:
  {{- include "telark.labels" $root | nindent 4 }}
Per-resource component labels (app.kubernetes.io/component) are added at the call
site, since they vary per service.
*/}}
{{- define "telark.labels" -}}
helm.sh/chart: {{ include "telark.chart" . }}
{{ include "telark.selectorLabels" (dict "root" .) }}
app.kubernetes.io/part-of: {{ include "telark.name" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Selector labels — the immutable subset. Call with a dict:
  {{- include "telark.selectorLabels" (dict "root" $root "component" $name) | nindent 6 }}
Omit "component" for the common (non-selector) label block.
*/}}
{{- define "telark.selectorLabels" -}}
app.kubernetes.io/name: {{ include "telark.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
{{- with .component }}
app.kubernetes.io/component: {{ . }}
{{- end }}
{{- end -}}

{{/*
Common annotations, applied to every resource when set.
  {{- include "telark.annotations" $root | nindent 4 }}
*/}}
{{- define "telark.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
ServiceAccount name for a service. Honours a per-service serviceAccount.name
override, otherwise <fullname>-<serviceName>-sa. Call with dict:
  {{- include "telark.serviceAccountName" (dict "root" $root "serviceConfig" $svc) }}
*/}}
{{- define "telark.serviceAccountName" -}}
{{- $sa := .serviceConfig.serviceAccount | default dict -}}
{{- if $sa.name -}}
{{- $sa.name -}}
{{- else -}}
{{- printf "%s-%s-sa" (include "telark.fullname" .root) .serviceConfig.name -}}
{{- end -}}
{{- end -}}

{{/*
Name of the exporter snapshots PVC. Single source for the PVC itself, the volume
claimName, and the SNAPSHOTS_PVC_NAME env. Call with the root context.
*/}}
{{- define "telark.exporterSnapshotsPvcName" -}}
{{- printf "%s-exporter-snapshots-pvc" (include "telark.fullname" .) -}}
{{- end -}}

{{/*
Name of the exporter reports PVC. Single source for the PVC itself and the
volume claimName. Call with the root context.
*/}}
{{- define "telark.exporterReportsPvcName" -}}
{{- printf "%s-exporter-reports-pvc" (include "telark.fullname" .) -}}
{{- end -}}

{{/*
imagePullSecrets block from global.imagePullSecrets + app.image.pullSecrets.
Renders nothing when both are empty (public images pull anonymously).
  {{- include "telark.imagePullSecrets" $root | nindent 6 }}
*/}}
{{- define "telark.imagePullSecrets" -}}
{{- $root := . -}}
{{- $secrets := list -}}
{{- with $root.Values.global -}}
{{- range .imagePullSecrets -}}
{{- $secrets = append $secrets (kindIs "string" . | ternary (dict "name" .) .) -}}
{{- end -}}
{{- end -}}
{{- range $root.Values.app.image.pullSecrets -}}
{{- $secrets = append $secrets (kindIs "string" . | ternary (dict "name" .) .) -}}
{{- end -}}
{{- if $secrets -}}
imagePullSecrets:
{{- range $secrets }}
  - name: {{ .name }}
{{- end }}
{{- end -}}
{{- end -}}

{{/*
Secret holding the service token: app.serviceToken.existingSecret when set,
otherwise the chart-generated one. Call with the root context.
*/}}
{{- define "telark.serviceTokenSecretName" -}}
{{- default (printf "%s-service-token-secret" .Values.app.name) .Values.app.serviceToken.existingSecret -}}
{{- end -}}

{{- define "telark.oidcTrustSecretName" -}}
{{- default (printf "%s-oidc-trust-secret" (include "telark.fullname" .)) .Values.app.auth.oidc.existingSecret -}}
{{- end -}}

{{/* Constant, not derived from app.name: the Go services hardcode it. */}}
{{- define "telark.apiGroup" -}}
telark.io
{{- end -}}

{{/*
Secret holding one NATS user's credentials (publisher or consumer):
nats.existingSecrets.<user> when set, otherwise the chart-generated one.
  {{- include "telark.natsSecretName" (dict "root" $root "user" "publisher") }}
*/}}
{{- define "telark.natsSecretName" -}}
{{- $user := required "services.<svc>.natsUser must be publisher or consumer when useNatsCreds is true" .user -}}
{{- $existing := index (.root.Values.nats.existingSecrets | default dict) $user -}}
{{- default (printf "%s-nats-%s-secret" .root.Values.app.name $user) $existing -}}
{{- end -}}
