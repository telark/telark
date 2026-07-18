{{/*
The exporter keeps snapshots on a filesystem. More than one replica therefore
needs a volume more than one node can mount at once, and a StorageClass that can
actually provide it.

Helm can only check what was declared, not what the cluster can honour. Declaring
ReadWriteMany against a block-storage class leaves the claim Pending rather than
running degraded, which is the failure we want: loud, at install time, instead of
a "performance" release quietly serving from a single replica.
*/}}
{{- define "validateExporterStorage" -}}
{{- $values := .values -}}
{{- $exporter := $values.services.exporter | default dict -}}
{{- if $exporter.enabled -}}
{{- $defaults := $values.app.serviceDefaults | default dict -}}
{{- $replicas := default $defaults.replicas $exporter.replicas | default 1 | int -}}
{{- $persistence := $values.app.persistence | default dict -}}
{{- $accessMode := $persistence.accessMode | default "ReadWriteOnce" -}}
{{- if gt $replicas 1 -}}
{{- if ne $accessMode "ReadWriteMany" -}}
{{- fail (printf "exporter is set to %d replicas but app.persistence.accessMode is %s. Snapshots live on that volume, so every replica past the first cannot mount it and stays Pending. Set app.persistence.accessMode=ReadWriteMany with a storage class that supports it, or set services.exporter.replicas=1." $replicas $accessMode) -}}
{{- end -}}
{{- if not $persistence.storageClass -}}
{{- fail (printf "exporter is set to %d replicas with accessMode=ReadWriteMany but app.persistence.storageClass is empty, so the cluster default is used. Defaults are block storage on every managed provider and cannot serve ReadWriteMany, so the claim would never bind. Name a class that supports it, for example an EFS class on EKS." $replicas) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
