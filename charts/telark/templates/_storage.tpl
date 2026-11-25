{{/*
The exporter's snapshots and reports claims. Kubernetes never changes a claim's class or
access mode, so a live claim stays while it fits (storageClass "" accepts any class, more
than one replica needs ReadWriteMany); otherwise the data moves to a claim named after the
requested storage (telark upgrade copies it first, see docs/INSTALL.md) and the old claim is kept. New claims
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
{{- $claims := dict -}}
{{- range $kind, $size := dict "snapshots" $persistence.snapshotsSize "reports" $persistence.reportsSize -}}
{{- $base := printf "%s-exporter-%s-pvc" $fullname $kind -}}
{{- $target := printf "%s%s" $base $suffix -}}
{{- /* The mounted claim, else (a reinstall) kept ones: the requested storage's, then the first install's. */}}
{{- $candidates := list $target $base -}}
{{- with get $mounted (printf "%s-storage" $kind) }}{{ $candidates = prepend $candidates . }}{{ end -}}
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
