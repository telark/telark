#!/usr/bin/env bash
set -euo pipefail

mkdir -p /tmp/kubeconform-cache
# shellcheck disable=SC2086 # RENDER_MODES / K8S_VERSIONS are deliberately space-separated lists
for mode in $RENDER_MODES; do
  rendered="/tmp/rendered-$mode.yaml"
  helm template t "$CHART_PATH" \
    --set app.mode="$mode" \
    --set app.persistence.storageClass="$STORAGE_CLASS" > "$rendered"

  for version in $K8S_VERSIONS; do
    echo "== kubeconform $mode · k8s $version =="
    kubeconform -strict -summary -ignore-missing-schemas \
      -kubernetes-version "$version" \
      -cache /tmp/kubeconform-cache \
      -schema-location default \
      -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' \
      < "$rendered"
  done
done
