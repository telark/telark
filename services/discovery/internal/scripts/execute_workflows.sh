#!/usr/bin/env bash
set -euo pipefail

OWNER="telark"
REPO="release-manager"
WORKFLOW_FILE="build-service.yaml"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

SERVICE="discovery"
BRANCH="feat/agent_changes"
BUILD_TYPE="${BUILD_TYPE:-}"
HELM_BUILD_TYPE="${HELM_BUILD_TYPE:-}"
RELEASE_MANAGER_CHANGED="${RELEASE_MANAGER_CHANGED:-}"

REF="main" # workflow ref (branch/tag in THIS repo)

usage() {
  cat <<'EOF'
Usage:
  scripts/execute_workflows.sh [--build-type <val>] [--helm-build-type <val>] [--release-manager-changed <true|false>] [--ref <branch>]

Defaults:
  - If no release-manager changes: build_type=re-build-current, helm_build_type=skip
  - If release-manager changed: build_type=patch, helm_build_type=patch

Overrides:
  - BUILD_TYPE / HELM_BUILD_TYPE env vars override defaults
  - RELEASE_MANAGER_CHANGED env var can be set to true/false
EOF
}

parse_bool() {
  # bash 3.x compatibility (macOS default) – avoid ${var,,}
  local v
  v="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')"
  case "${v}" in
    1|true|yes|y) echo "true" ;;
    0|false|no|n|"") echo "false" ;;
    *) echo "invalid" ;;
  esac
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --build-type) BUILD_TYPE="${2:-}"; shift 2 ;;
      --helm-build-type) HELM_BUILD_TYPE="${2:-}"; shift 2 ;;
      --release-manager-changed) RELEASE_MANAGER_CHANGED="${2:-}"; shift 2 ;;
      --ref) REF="${2:-}"; shift 2 ;;
      -h|--help) usage; exit 0 ;;
      *)
        echo "Unknown arg: $1" >&2
        usage >&2
        exit 2
        ;;
    esac
  done
}

parse_args "$@"

rm_changed="$(parse_bool "${RELEASE_MANAGER_CHANGED}")"
if [[ "${rm_changed}" == "invalid" ]]; then
  echo "Invalid --release-manager-changed value: ${RELEASE_MANAGER_CHANGED}" >&2
  exit 2
fi

if [[ -z "${BUILD_TYPE}" || -z "${HELM_BUILD_TYPE}" ]]; then
  if [[ "${rm_changed}" == "true" ]]; then
    : "${BUILD_TYPE:=patch}"
    : "${HELM_BUILD_TYPE:=patch}"
  else
    : "${BUILD_TYPE:=re-build-current}"
    : "${HELM_BUILD_TYPE:=skip}"
  fi
fi

api() {
  local token="${GITHUB_TOKEN}"
  if [[ -z "${token}" ]]; then
    echo "GITHUB_TOKEN is required but not set." >&2
    exit 2
  fi
  curl -sS -L \
    --connect-timeout 5 \
    --max-time 30 \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${token}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$@"
}

DISPATCH_TS="$(date -u +%s)"

# 1) dispatch (likely returns 204, no body)
echo "Dispatching workflow: repo=${OWNER}/${REPO} workflow=${WORKFLOW_FILE} ref=${REF} service=${SERVICE} branch=${BRANCH} build_type=${BUILD_TYPE} helm_build_type=${HELM_BUILD_TYPE}"
api -X POST \
  "https://api.github.com/repos/${OWNER}/${REPO}/actions/workflows/${WORKFLOW_FILE}/dispatches" \
  -d "{
    \"ref\": \"${REF}\",
    \"inputs\": {
      \"service\": \"${SERVICE}\",
      \"branch\": \"${BRANCH}\",
      \"build_type\": \"${BUILD_TYPE}\",
      \"helm_build_type\": \"${HELM_BUILD_TYPE}\"
    }
  }" >/dev/null

# 2) discover run id by listing recent runs for that workflow
echo "Discovering run id..."
RUN_ID=""
for _ in {1..30}; do
  RUN_ID="$(
    api "https://api.github.com/repos/${OWNER}/${REPO}/actions/workflows/${WORKFLOW_FILE}/runs?event=workflow_dispatch&branch=${REF}&per_page=20" |
      jq -r --argjson t "${DISPATCH_TS}" --arg svc "${SERVICE}" --arg bt "${BUILD_TYPE}" '
        .workflow_runs
        | map(select((.created_at | fromdateiso8601) >= ($t - 5)))
        | map(select((.display_title // .name // "") | test($svc)))
        | map(select((.display_title // .name // "") | test($bt)))
        | first
        | .id // empty
      '
  )"
  [[ -n "${RUN_ID}" ]] && break
  sleep 2
done

if [[ -z "${RUN_ID}" ]]; then
  echo "Failed to discover run_id after dispatch."
  exit 1
fi

echo "Run id: ${RUN_ID}"

# 3) poll run status until completion
while :; do
  RUN_JSON="$(api "https://api.github.com/repos/${OWNER}/${REPO}/actions/runs/${RUN_ID}")"
  STATUS="$(jq -r '.status' <<<"${RUN_JSON}")"
  CONCLUSION="$(jq -r '.conclusion' <<<"${RUN_JSON}")"
  echo "Workflow status: status=${STATUS} conclusion=${CONCLUSION}"

  if [[ "${STATUS}" == "completed" ]]; then
    if [[ "${CONCLUSION}" == "success" ]]; then
      echo "DONE"
      exit 0
    fi
    echo "Workflow completed but conclusion=${CONCLUSION}"
    exit 1
  fi

  sleep 5
done