#!/usr/bin/env bash
set -euo pipefail

floor="$(sed -n 's/^kubeVersion:.*>=\([0-9]*\.[0-9]*\).*/\1/p' "$CHART_PATH/Chart.yaml")"
if [[ -z "$floor" ]]; then
  echo "could not read a '>=' kubeVersion floor from $CHART_PATH/Chart.yaml"; exit 1
fi
# shellcheck disable=SC2086 # K8S_VERSIONS is a deliberately space-separated list
for version in $K8S_VERSIONS; do
  case "$version" in "$floor".*) exit 0 ;; *) continue ;; esac
done
echo "chart supports Kubernetes >= $floor but that version is not in: $K8S_VERSIONS"
echo "update the kubernetes-versions input to match Chart.yaml."
exit 1
