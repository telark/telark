{{/*
The exporter's snapshots and reports claims. Kubernetes never changes a claim's class or
access mode, so a live claim stays while it fits (storageClass "" accepts any class, more
than one replica needs ReadWriteMany); otherwise the data moves to a claim named after the
requested storage (telark.exporterMigration copies it) and the old claim is kept. New claims
take the base name, as cluster-less renders always do. A claim that exists keeps its spec;
its size only grows, and only where the class allows it.
Callers: {{- $storage := include "telark.exporterStorage" $ | fromYaml -}}
*/}}
{{- define "telark.exporterStorage" -}}
{{- $values := include "telark.modeValues" . | fromYaml -}}
{{- $persistence := $values.app.persistence -}}
{{- $exporter := $values.services.exporter -}}
{{- $ns := $values.app.namespace -}}
{{- $fullname := include "telark.fullname" . -}}
{{- $class := $persistence.storageClass | default "" -}}
{{- $className := ternary "" $class (eq $class "-") -}}
{{- $rwx := gt (int $exporter.replicas) 1 -}}
{{- $suffix := "" -}}
{{- if $class }}{{ $suffix = printf "-%s" (ternary "noclass" $class (eq $class "-")) }}{{ end -}}
{{- if $rwx }}{{ $suffix = printf "%s-rwx" $suffix }}{{ end -}}
{{- $deployment := lookup "apps/v1" "Deployment" $ns (printf "%s-%s" $fullname $exporter.name) -}}
{{- $mounted := dict -}}
{{- range (dig "spec" "template" "spec" "volumes" list $deployment) -}}
{{- if .persistentVolumeClaim }}{{ $_ := set $mounted .name .persistentVolumeClaim.claimName }}{{ end -}}
{{- end -}}
{{- $replicas := dig "spec" "replicas" 0 $deployment | int64 -}}
{{- $settled := and (eq (dig "status" "observedGeneration" -1 $deployment | int64) (dig "metadata" "generation" 0 $deployment | int64)) (eq (dig "status" "updatedReplicas" 0 $deployment | int64) $replicas) (eq (dig "status" "readyReplicas" 0 $deployment | int64) $replicas) -}}
{{- $claims := dict -}}
{{- range $kind, $size := dict "snapshots" $persistence.snapshotsSize "reports" $persistence.reportsSize -}}
{{- $base := printf "%s-exporter-%s-pvc" $fullname $kind -}}
{{- $target := printf "%s%s" $base $suffix -}}
{{- /* The mounted claim, else (a reinstall) kept ones: the requested storage's, then the first install's. A copy whose rollout never finished starts over from the claim that still holds the data. */}}
{{- $candidates := list $target $base -}}
{{- with get $mounted (printf "%s-storage" $kind) }}{{ $candidates = prepend $candidates . }}{{ end -}}
{{- $previous := get $mounted (printf "previous-%s" $kind) -}}
{{- if and $previous (not $settled) }}{{ $candidates = prepend $candidates $previous }}{{ end -}}
{{- $livePvc := dict -}}
{{- range $candidates -}}
{{- if not $livePvc }}{{ $livePvc = lookup "v1" "PersistentVolumeClaim" $ns . }}{{ end -}}
{{- end -}}
{{- $live := dig "metadata" "name" $base $livePvc -}}
{{- $fits := or (not $livePvc) (and (or (not $class) (eq (dig "spec" "storageClassName" "" $livePvc) $className)) (or (not $rwx) (has "ReadWriteMany" (dig "spec" "accessModes" list $livePvc)))) -}}
{{- $claim := dict "name" $live "size" $size -}}
{{- if and (not $fits) (ne $live $target) -}}
{{- $_ := set $claim "name" $target -}}
{{- $from := dict "name" $live "accessModes" $livePvc.spec.accessModes "size" $livePvc.spec.resources.requests.storage -}}
{{- if hasKey $livePvc.spec "storageClassName" }}{{ $_ := set $from "storageClassName" $livePvc.spec.storageClassName }}{{ end -}}
{{- $_ := set $claim "from" $from -}}
{{- end -}}
{{- $current := $livePvc -}}
{{- if ne $claim.name $live }}{{ $current = lookup "v1" "PersistentVolumeClaim" $ns $claim.name }}{{ end -}}
{{- if $current -}}
{{- $_ := set $claim "accessModes" $current.spec.accessModes -}}
{{- if hasKey $current.spec "storageClassName" }}{{ $_ := set $claim "storageClassName" $current.spec.storageClassName }}{{ end -}}
{{- $liveSize := $current.spec.resources.requests.storage -}}
{{- $expandable := and $current.spec.storageClassName (dig "allowVolumeExpansion" false (lookup "storage.k8s.io/v1" "StorageClass" "" (toString $current.spec.storageClassName))) -}}
{{- if not (and $expandable (gt (include "telark.quantityBytes" $size | float64) (include "telark.quantityBytes" $liveSize | float64))) }}{{ $_ := set $claim "size" $liveSize }}{{ end -}}
{{- else -}}
{{- $_ := set $claim "accessModes" (list (ternary "ReadWriteMany" "ReadWriteOnce" $rwx)) -}}
{{- if $class }}{{ $_ := set $claim "storageClassName" $className }}{{ end -}}
{{- end -}}
{{- $_ := set $claims $kind $claim -}}
{{- end -}}
{{- toYaml (dict "claims" $claims) -}}
{{- end -}}

{{- define "telark.quantityBytes" -}}
{{- $number := regexFind "^[0-9.]+" . -}}
{{- $scale := get (dict "k" 1e3 "M" 1e6 "G" 1e9 "T" 1e12 "P" 1e15 "Ki" 1024.0 "Mi" 1048576.0 "Gi" 1073741824.0 "Ti" 1099511627776.0 "Pi" 1125899906842624.0) (trimPrefix $number .) | default 1.0 -}}
{{- mulf (float64 $number) $scale -}}
{{- end -}}

{{/*
While a claim moves, the exporter's init container copies the old claim into the new one.
Recreate keeps old pods from writing during the copy; an old ReadWriteOnce claim attaches to
one node only, so every replica runs there until the next upgrade drops the old claim.
Mutates .values.services.exporter; call before rendering the exporter Deployment.
*/}}
{{- define "telark.exporterMigration" -}}
{{- $values := .values -}}
{{- $exporter := $values.services.exporter -}}
{{- $storage := include "telark.exporterStorage" .root | fromYaml -}}
{{- $volumes := $exporter.volumes -}}
{{- $mounts := list -}}
{{- $args := list -}}
{{- $readWriteOnce := false -}}
{{- range $mount := $exporter.volumeMounts -}}
{{- $kind := trimSuffix "-storage" $mount.name -}}
{{- $from := dig "claims" $kind "from" dict $storage -}}
{{- if $from -}}
{{- $previous := printf "previous-%s" $kind -}}
{{- $volumes = append $volumes (dict "name" $previous "persistentVolumeClaim" (dict "claimName" $from.name)) -}}
{{- $mounts = concat $mounts (list (dict "name" $previous "mountPath" (printf "/previous%s" $mount.path) "readOnly" true) (dict "name" $mount.name "mountPath" $mount.path)) -}}
{{- $args = concat $args (list (printf "/previous%s" $mount.path) $mount.path) -}}
{{- if not (has "ReadWriteMany" $from.accessModes) }}{{ $readWriteOnce = true }}{{ end -}}
{{- end -}}
{{- end -}}
{{- if $args -}}
{{- $_ := set $exporter "volumes" $volumes -}}
{{- $_ := set $exporter "strategy" "Recreate" -}}
{{- $_ := set $exporter "initContainers" (list (dict
  "name" "migrate-storage"
  "image" (printf "%s/%s:%s" $values.app.image.registry $exporter.repository (toString $exporter.version))
  "imagePullPolicy" ($values.app.image.pullPolicy | default "Always")
  "args" (prepend $args "migrate-storage")
  "volumeMounts" $mounts
  "resources" $values.app.shared.resources
  "securityContext" (dict "allowPrivilegeEscalation" false "readOnlyRootFilesystem" true "capabilities" (dict "drop" (list "ALL"))))) -}}
{{- if and $readWriteOnce (gt (int $exporter.replicas) 1) -}}
{{- $affinity := deepCopy ($exporter.affinity | default $values.app.serviceDefaults.affinity | default dict) -}}
{{- $selector := include "telark.selectorLabels" (dict "root" .root "component" $exporter.name) | fromYaml -}}
{{- $_ := set $affinity "podAffinity" (dict "requiredDuringSchedulingIgnoredDuringExecution" (list (dict "labelSelector" (dict "matchLabels" $selector) "topologyKey" "kubernetes.io/hostname"))) -}}
{{- $_ := set $exporter "affinity" $affinity -}}
{{- end -}}
{{- end -}}
{{- end -}}
