{{/*
The exporter keeps snapshots on a filesystem. More than one replica therefore
needs a volume more than one node can mount at once (telark.modeValues already
set ReadWriteMany), and a StorageClass that can actually provide it.

Helm can only check what was declared, not what the cluster can honour. The
cluster default class is block storage on every managed provider, so a
ReadWriteMany claim against it stays Pending forever. Fail loud at install time
instead of leaving a release quietly stuck.
*/}}
{{- define "validateExporterStorage" -}}
{{- $values := .values -}}
{{- $exporter := $values.services.exporter | default dict -}}
{{- if $exporter.enabled -}}
{{- $replicas := $exporter.replicas | default 1 | int -}}
{{- $persistence := $values.app.persistence | default dict -}}
{{- if and (gt $replicas 1) (not $persistence.storageClass) -}}
{{- fail (printf "exporter runs %d replicas on a ReadWriteMany volume but app.persistence.storageClass is empty, so the cluster default (block storage) would be used and the claim would never bind. Set app.persistence.storageClass to a ReadWriteMany class (for example efs-sc on EKS), or --set app.singleNode=true to run one replica on ReadWriteOnce." $replicas) -}}
{{- end -}}
{{- end -}}
{{- end -}}
