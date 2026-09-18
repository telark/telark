{{/*
Resolve the effective values tree for an install mode. app.mode (default
standard) selects a preset under modes/<mode>.yaml that is deep-merged over the
chart values; the preset wins, so --set app.mode=performance sizes every telark
service from one flag. standard is the chart baseline, so it needs no preset.
The preset sizes telark's own workloads only — a subchart's values cannot be set
from here, so redis/nats/kyverno keep the chart defaults regardless of mode.

The exporter's update strategy and the snapshot volume's access mode follow its
replica count (app.singleNode forces 1), so every template that reads them sees
one consistent answer: >1 replica → RollingUpdate + ReadWriteMany, else
Recreate + ReadWriteOnce.
Callers: {{- $values := include "telark.modeValues" $ | fromYaml -}}
*/}}
{{- define "telark.modeValues" -}}
{{- $mode := .Values.app.mode | default "standard" -}}
{{- $v := deepCopy .Values -}}
{{- if ne $mode "standard" -}}
{{- $overlay := .Files.Get (printf "modes/%s.yaml" $mode) | fromYaml -}}
{{- if not $overlay -}}
{{- fail (printf "app.mode=%q is not a preset — use minimal, standard or performance" $mode) -}}
{{- end -}}
{{- $v = mergeOverwrite $v $overlay -}}
{{- end -}}
{{- $exporter := $v.services.exporter -}}
{{- $replicas := $exporter.replicas | default $v.app.serviceDefaults.replicas | default 1 | int -}}
{{- if $v.app.singleNode -}}
{{- $replicas = 1 -}}
{{- end -}}
{{- $_ := set $exporter "replicas" $replicas -}}
{{- $_ := set $exporter "strategy" (ternary "RollingUpdate" "Recreate" (gt $replicas 1)) -}}
{{- $_ := set $v.app.persistence "accessMode" (ternary "ReadWriteMany" "ReadWriteOnce" (gt $replicas 1)) -}}
{{- toYaml $v -}}
{{- end -}}
