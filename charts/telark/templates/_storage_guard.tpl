{{/*
Exporter replicas share both volumes, so more than one needs a class that serves
ReadWriteMany; the cluster default is block storage on every managed provider and such a
claim never binds. A class the cluster lacks fails here too instead of leaving a claim
Pending (only with cluster access: helm template cannot look it up).
*/}}
{{- define "validateExporterStorage" -}}
{{- $values := .values -}}
{{- $exporter := $values.services.exporter | default dict -}}
{{- $persistence := $values.app.persistence | default dict -}}
{{- if and $exporter.enabled $persistence.enabled -}}
{{- $replicas := $exporter.replicas | default 1 | int -}}
{{- $class := $persistence.storageClass | default "" -}}
{{- if and (gt $replicas 1) (not $class) -}}
{{- fail (printf "services.exporter.replicas=%d share the exporter's volumes, so they need a ReadWriteMany class: set app.persistence.storageClass to one (for example efs-sc on EKS), or keep one replica." $replicas) -}}
{{- end -}}
{{- if and $class (ne $class "-") (lookup "v1" "Namespace" "" "kube-system") (not (lookup "storage.k8s.io/v1" "StorageClass" "" $class)) -}}
{{- fail (printf "app.persistence.storageClass=%q does not exist in this cluster (kubectl get storageclass lists them)." $class) -}}
{{- end -}}
{{- end -}}
{{- end -}}
