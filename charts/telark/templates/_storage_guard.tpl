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

{{/*
Kubernetes never changes a StatefulSet's volume claim template nor shrinks a claim, so a new Redis,
NATS or model volume size would fail the upgrade after other objects changed; stop before that.
*/}}
{{- define "validateSubchartVolumes" -}}
{{- $changed := list -}}
{{- $names := list -}}
{{- range $name, $persistence := dict (printf "%s-redis-master" .Release.Name) .Values.redis.master.persistence (printf "%s-nats" .Release.Name) .Values.nats.persistence -}}
{{- range (dig "spec" "volumeClaimTemplates" list (lookup "apps/v1" "StatefulSet" $.Release.Namespace $name)) -}}
{{- $live := .spec.resources.requests.storage -}}
{{- if and $persistence.enabled (ne (include "telark.quantityBytes" $live) (include "telark.quantityBytes" (toString $persistence.size))) -}}
{{- $changed = append $changed (printf "%s from %s to %s" $name $live $persistence.size) -}}
{{- $names = append $names $name -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if $changed -}}
{{- fail (printf "Kubernetes never changes the volume size of an existing StatefulSet, and this upgrade changes it (%s). Re-create the StatefulSets once, then upgrade again: kubectl delete statefulset -n %s %s --cascade=orphan. Their pods, claims and data stay, and the claims keep their size. To keep the old size instead, pass it on every upgrade (redis.master.persistence.size, nats.persistence.size)." (join ", " $changed) .Release.Namespace (join " " $names)) -}}
{{- end -}}
{{- $models := .Values.ollama.persistentVolume -}}
{{- if and .Values.app.ollama.enabled $models.enabled (not $models.existingClaim) -}}
{{- $claim := printf "%s-ollama" .Release.Name -}}
{{- with (lookup "v1" "PersistentVolumeClaim" .Release.Namespace $claim) -}}
{{- $live := .spec.resources.requests.storage -}}
{{- if gt (include "telark.quantityBytes" $live | float64) (include "telark.quantityBytes" (toString $models.size) | float64) -}}
{{- fail (printf "Kubernetes never shrinks a claim: %s has %s and the chart asks for %s. Keep its size on every upgrade: --set ollama.persistentVolume.size=%s" $claim $live $models.size $live) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}
