#!/usr/bin/env bash
set -euo pipefail

# usage: ./build-package.sh <package> [branch] [build_type]
# example: ./build-package.sh kcore main patch

OWNER="telark"
RELEASE_REPO="release-manager"
WORKFLOW_FILE="build-package.yaml"
TOKEN="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"

PKG="${1:-}"
BRANCH="${2:-main}"
BUILD_TYPE="${3:-patch}"

if [[ -z "${PKG}" ]]; then
  echo "Usage: $0 <package> [branch] [build_type]" >&2
  exit 2
fi

case "${PKG}" in
  data|rest|composer|kcore|x-ware) ;;
  *)
    echo "Invalid package: ${PKG}. Allowed: data, rest, composer, kcore, x-ware" >&2
    exit 2
    ;;
esac

api() {
  curl -sS -L \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$@"
}

DISPATCH_TS="$(date -u +%s)"

# 1) dispatch workflow
api -X POST \
  "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/workflows/${WORKFLOW_FILE}/dispatches" \
  -d "{
    \"ref\": \"main\",
    \"inputs\": {
      \"package\": \"${PKG}\",
      \"branch\": \"${BRANCH}\",
      \"build_type\": \"${BUILD_TYPE}\"
    }
  }" >/dev/null

# 2) discover run_id
RUN_ID=""
for _ in {1..30}; do
  RUN_ID="$(
    api "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/workflows/${WORKFLOW_FILE}/runs?event=workflow_dispatch&branch=main&per_page=20" |
      jq -r --argjson t "${DISPATCH_TS}" --arg pkg "${PKG}" --arg bt "${BUILD_TYPE}" '
        .workflow_runs
        | map(select((.created_at | fromdateiso8601) >= ($t - 5)))
        | map(select((.display_title // .name // "") | test($pkg)))
        | map(select((.display_title // .name // "") | test($bt)))
        | first
        | .id // empty
      '
  )"
  [[ -n "${RUN_ID}" ]] && break
  sleep 2
done

if [[ -z "${RUN_ID}" ]]; then
  echo "Failed to discover run_id after dispatch." >&2
  exit 1
fi

# 3) wait for completion
while :; do
  RUN_JSON="$(api "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/runs/${RUN_ID}")"
  STATUS="$(jq -r '.status' <<<"${RUN_JSON}")"
  CONCLUSION="$(jq -r '.conclusion' <<<"${RUN_JSON}")"

  if [[ "${STATUS}" == "completed" ]]; then
    [[ "${CONCLUSION}" == "success" ]] || { echo "completed but conclusion=${CONCLUSION}" >&2; exit 1; }
    break
  fi

  sleep 5
done

# 4) output "new version" = latest tag on telark/<package>
NEW_TAG="$(
  api "https://api.github.com/repos/${OWNER}/${PKG}/tags?per_page=1" | jq -r '.[0].name // empty'
)"

if [[ -z "${NEW_TAG}" ]]; then
  echo "Workflow succeeded, but failed to read latest tag from ${OWNER}/${PKG}." >&2
  exit 1
fi

echo "${NEW_TAG}"