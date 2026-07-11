#!/usr/bin/env bash
# Triggers "Build & Install Helm Release" (manual-build-release.yaml) via GitHub API.
# Usage: ./manual-build-release.sh <build_type> [sonar_scan]
#   build_type: patch | minor | major
#   sonar_scan: allow | skip   (default: skip)

set -euo pipefail

OWNER="telark"
RELEASE_REPO="release-manager"
WORKFLOW_FILE="manual-build-release.yaml"
REF="main"

TOKEN="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"

BUILD_TYPE="${1:-}"
SONAR_SCAN="${2:-skip}"

usage() {
  echo "Usage: $0 <build_type> [sonar_scan]" >&2
  echo "  build_type: patch | minor | major" >&2
  echo "  sonar_scan: allow | skip  (default: skip)" >&2
}

if [[ -z "${TOKEN}" ]]; then
  echo "Set GITHUB_TOKEN (PAT with repo scope for workflow dispatch)." >&2
  exit 2
fi

if [[ -z "${BUILD_TYPE}" ]]; then
  usage
  exit 2
fi

case "${BUILD_TYPE}" in
  patch|minor|major) ;;
  *)
    echo "Invalid build_type: ${BUILD_TYPE}" >&2
    usage
    exit 2
    ;;
esac

case "${SONAR_SCAN}" in
  allow|skip) ;;
  *)
    echo "Invalid sonar_scan: ${SONAR_SCAN}" >&2
    usage
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

api -X POST \
  "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/workflows/${WORKFLOW_FILE}/dispatches" \
  -d "{
    \"ref\": \"${REF}\",
    \"inputs\": {
      \"build_type\": \"${BUILD_TYPE}\",
      \"sonar_scan\": \"${SONAR_SCAN}\"
    }
  }" >/dev/null

RUN_ID=""
for _ in {1..30}; do
  RUN_ID="$(
    api "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/workflows/${WORKFLOW_FILE}/runs?event=workflow_dispatch&branch=${REF}&per_page=20" |
      jq -r --argjson t "${DISPATCH_TS}" --arg bt "${BUILD_TYPE}" '
        .workflow_runs
        | map(select((.created_at | fromdateiso8601) >= ($t - 5)))
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

while :; do
  RUN_JSON="$(api "https://api.github.com/repos/${OWNER}/${RELEASE_REPO}/actions/runs/${RUN_ID}")"
  STATUS="$(jq -r '.status' <<<"${RUN_JSON}")"
  CONCLUSION="$(jq -r '.conclusion' <<<"${RUN_JSON}")"

  if [[ "${STATUS}" == "completed" ]]; then
    if [[ "${CONCLUSION}" == "success" ]]; then
      echo "DONE"
      exit 0
    fi
    echo "Workflow completed but conclusion=${CONCLUSION}" >&2
    exit 1
  fi

  sleep 5
done
