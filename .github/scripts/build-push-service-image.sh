#!/usr/bin/env bash
set -euo pipefail

start_time=$(date +%s)

echo "🐳 Building Docker image for service: $SERVICE_NAME"
echo "📦 Image tag: $IMAGE_TAG"
echo "🏗️  Platforms: $PLATFORMS"

build_cmd=(docker buildx build)

# GitHub Actions cache, scoped per service so images stay out of the
# public registry (no cache-* tags on GHCR).
if [[ "$CACHE_ENABLED" == "true" ]]; then
  build_cmd+=(
    "--cache-from=type=gha,scope=$SERVICE_NAME"
    "--cache-to=type=gha,scope=$SERVICE_NAME,mode=max"
  )
fi

build_cmd+=(
  "--platform=$PLATFORMS"
  --sbom=true
  --provenance=mode=max
  --progress=plain
  --metadata-file=/tmp/metadata.json
)

# Command substitution, not `< <(jq ...)`: a malformed BUILD_ARGS must still
# abort the step, and a process substitution's exit status is not checked.
if [[ "$BUILD_ARGS" != "{}" ]]; then
  additional_args=$(jq -r 'to_entries[] | "\(.key)=\(.value)"' <<< "$BUILD_ARGS")
  while IFS= read -r arg; do
    if [[ -n "$arg" ]]; then build_cmd+=(--build-arg "$arg"); fi
  done <<< "$additional_args"
fi

image_name="$CONTAINER_REGISTRY_NAME/$CONTAINER_REGISTRY_REPO_NAME:$IMAGE_TAG"
build_cmd+=(-f "$DOCKERFILE_PATH" -t "$image_name")

[[ "$PUSH_ENABLED" == "true" ]] && build_cmd+=(--push)

build_cmd+=("$SERVICE_PATH/")

echo "🚀 Executing build command..."
"${build_cmd[@]}"

end_time=$(date +%s)
build_time=$((end_time - start_time))

{
  echo "image_name=$image_name"
  echo "build_time=$build_time"
} >> "$GITHUB_OUTPUT"

if [[ ! -f "/tmp/metadata.json" ]]; then
  echo "❌ Buildx metadata file was not created"
  exit 1
fi

echo "📄 Buildx metadata:"
jq . /tmp/metadata.json

image_digest=$(jq -r '.["containerimage.digest"] // empty' /tmp/metadata.json)

if [[ -z "$image_digest" ]]; then
  echo "❌ No container image digest found in Buildx metadata"
  exit 1
fi

echo "image_digest=$image_digest" >> "$GITHUB_OUTPUT"
echo "🔐 Image digest: $image_digest"

echo "✅ Build completed successfully!"
echo "⏱️  Build time: ${build_time}s"
echo "🏷️  Image: $image_name"
