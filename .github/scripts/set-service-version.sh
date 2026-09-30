#!/usr/bin/env bash
set -euo pipefail

# SERVICE_NEW_VERSION comes from the semver-bump action; this only records it.
sed -i "/${SERVICE_NAME}:/,/version: /s/version: $SERVICE_CURRENT_VERSION/version: $SERVICE_NEW_VERSION/" "$VALUES_FILE_PATH"

{
  echo "service_name=$SERVICE_NAME"
  echo "service_new_version=$SERVICE_NEW_VERSION"
  echo "new_version=$SERVICE_NEW_VERSION"
  echo "service_current_version=$SERVICE_CURRENT_VERSION"
} >> "$GITHUB_OUTPUT"

echo "✅ Version bumped for service: $SERVICE_NAME"
echo "   Current: $SERVICE_CURRENT_VERSION → New: $SERVICE_NEW_VERSION"
echo "   Build type: $BUILD_TYPE"
echo "   Image tag: $SERVICE_NEW_VERSION"
