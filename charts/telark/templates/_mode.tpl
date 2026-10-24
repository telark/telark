{{/*
Resolve the effective values tree for an install mode. app.mode (default
standard) selects a preset under modes/<mode>.yaml that is deep-merged over the
chart values; the preset wins, so --set app.mode=performance sizes every telark
service from one flag. standard is the chart baseline, so it needs no preset.
The preset sizes telark's own workloads only — a subchart's values cannot be set
from here, so redis/nats/kyverno keep the chart defaults regardless of mode.

The exporter's update strategy follows its replica count: >1 replica →
RollingUpdate on shared ReadWriteMany volumes (telark.exporterStorage), else
Recreate. A disruption budget on one replica would block every node drain.
Callers: {{- $values := include "telark.modeValues" $ | fromYaml -}}
*/}}
{{- define "telark.modeValues" -}}
{{- $mode := .Values.app.mode | default "standard" -}}
{{- $v := deepCopy .Values -}}
{{- /* Keys moved in 0.2.0: only a user file, --set or --reuse-values can still hold the old path, so it wins. A later release drops it. */ -}}
{{- $security := $v.app.security | default dict -}}
{{- $_ := set $v.app "security" $security -}}
{{- range $key := list "crdGuard" "networkPolicy" "serviceToken" -}}
{{- with index $v.app $key }}{{ $_ := set $security $key (mergeOverwrite (index $security $key | default dict) .) }}{{ end -}}
{{- end -}}
{{- with $v.app.kubectlImage }}{{ $_ := set $v.app.image "kubectl" . }}{{ end -}}
{{- range $dep := list "kyverno" "ollama" -}}
{{- $old := index $v.app $dep | default dict -}}
{{- if hasKey $old "enabled" }}{{ $_ := set (index $v $dep) "enabled" $old.enabled }}{{ end -}}
{{- end -}}
{{- if ne $mode "standard" -}}
{{- $overlay := .Files.Get (printf "modes/%s.yaml" $mode) | fromYaml -}}
{{- if not $overlay -}}
{{- fail (printf "app.mode=%q is not a preset — use minimal, standard or performance" $mode) -}}
{{- end -}}
{{- $v = mergeOverwrite $v $overlay -}}
{{- end -}}
{{- $exporter := $v.services.exporter -}}
{{- $replicas := $exporter.replicas | default $v.app.serviceDefaults.replicas | default 1 | int -}}
{{- $_ := set $exporter "replicas" $replicas -}}
{{- $_ := set $exporter "strategy" (ternary "RollingUpdate" "Recreate" (gt $replicas 1)) -}}
{{- if le $replicas 1 }}{{ $_ := set $exporter "pdb" (dict "enabled" false) }}{{ end -}}
{{- toYaml $v -}}
{{- end -}}
