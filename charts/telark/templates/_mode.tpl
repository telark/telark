{{/*
Resolve the effective values tree for an install mode. app.mode (default
standard) selects a preset under modes/<mode>.yaml that is deep-merged over the
chart values; the preset wins, so --set app.mode=performance sizes every telark
service from one flag. standard is the chart baseline, so it needs no preset.
The preset sizes telark's own workloads only — a subchart's values cannot be set
from here, so redis/nats/kyverno keep the chart defaults regardless of mode.
Callers: {{- $values := include "telark.modeValues" $ | fromYaml -}}
*/}}
{{- define "telark.modeValues" -}}
{{- $mode := .Values.app.mode | default "standard" -}}
{{- if eq $mode "standard" -}}
{{- toYaml .Values -}}
{{- else -}}
{{- $overlay := .Files.Get (printf "modes/%s.yaml" $mode) | fromYaml -}}
{{- if not $overlay -}}
{{- fail (printf "app.mode=%q is not a preset — use minimal, standard or performance" $mode) -}}
{{- end -}}
{{- mergeOverwrite (deepCopy .Values) $overlay | toYaml -}}
{{- end -}}
{{- end -}}
