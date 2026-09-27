#!/usr/bin/env bash
set -euo pipefail

# Convert kebab-case to camelCase (e.g. "demo-demo" -> "demoDemo"); the Helm
# values key is camelCase while the workflow input is kebab-case.
if [[ "$SERVICE" == *"-"* ]]; then
  service_name=$(echo "$SERVICE" | awk -F'-' '{for (i=2; i<=NF; i++) $i=toupper(substr($i,1,1)) substr($i,2)} 1' OFS='')
else
  service_name="$SERVICE"
fi

service_current_version=$(SERVICE_NAME="$service_name" yq '.services[strenv(SERVICE_NAME)].version' "$VALUES_FILE_PATH")

case "$BUILD_TYPE" in
  major)
    service_new_version=$(echo "$service_current_version" | awk -F. '{$1+=1;$2=0;$3=0;OFS=".";print $1,$2,$3}')
    ;;
  minor)
    service_new_version=$(echo "$service_current_version" | awk -F. '{$2+=1;$3=0;OFS=".";print $1,$2,$3}')
    ;;
  patch)
    service_new_version=$(echo "$service_current_version" | awk -F. '{$3+=1;OFS=".";print $1,$2,$3}')
    ;;
  *)
    echo "Invalid version type: $BUILD_TYPE"
    echo "Valid options: major, minor, patch"
    exit 1
    ;;
esac

sed -i "/${service_name}:/,/version: /s/version: $service_current_version/version: $service_new_version/" "$VALUES_FILE_PATH"

{
  echo "service_name=$service_name"
  echo "service_new_version=$service_new_version"
  echo "new_version=$service_new_version"
  echo "service_current_version=$service_current_version"
} >> "$GITHUB_OUTPUT"

echo "✅ Version bumped for service: $service_name"
echo "   Current: $service_current_version → New: $service_new_version"
echo "   Build type: $BUILD_TYPE"
echo "   Image tag: $service_new_version"
