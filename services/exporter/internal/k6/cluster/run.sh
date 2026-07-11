#!/usr/bin/env bash
#
# Run one k6 scenario as a K8s Job in the telark namespace.
#
# Lifecycle per invocation:
#   1. Flatten k6 scripts → temp dir (ConfigMap mount needs flat layout).
#   2. Create per-run ConfigMap with the flattened scripts.
#   3. Render Job manifest from job.yaml template.
#   4. Apply Job. Wait for pod. Stream logs.
#   5. Wait for Job completion. Copy result files out of the pod.
#   6. Delete Job + ConfigMap (always, via EXIT trap).
#
# Usage:
#   k6/cluster/run.sh <smoke|load|stress|spike|journey|soak> [namespace]
#
# Env overrides (all optional):
#   BASE_URL          default http://telark-exporter-service.telark.svc.cluster.local:8080
#   BASELINE_RPS      default 500
#   MAX_RPS           default 2000
#   SEED_POOL_SIZE    default 1000
#   SEED_PREFIX       default k6-<run-tag>
#   WAIT_TIMEOUT      default 30m
#   IMAGE             default grafana/k6:latest

set -euo pipefail

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

readonly VALID_SCENARIOS=(smoke load stress spike journey soak)
readonly DEFAULT_BASE_URL="http://telark-exporter-service.telark.svc.cluster.local:8080"
readonly DEFAULT_BASELINE_RPS=500
readonly DEFAULT_MAX_RPS=2000
readonly DEFAULT_SEED_POOL_SIZE=1000
readonly DEFAULT_WAIT_TIMEOUT="30m"
readonly DEFAULT_IMAGE="grafana/k6:latest"
readonly POD_APPEAR_TIMEOUT_SECONDS=60
readonly POD_RUNNING_TIMEOUT="2m"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

log() { echo "[$1] ${*:2}"; }
die() { echo "ERROR: $*" >&2; exit 1; }

is_valid_scenario() {
  local name="$1"
  for v in "${VALID_SCENARIOS[@]}"; do
    [[ "$v" == "$name" ]] && return 0
  done
  return 1
}

# In-place sed that works on both macOS and Linux.
sed_inplace() {
  if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "$@"
  else
    sed -i "$@"
  fi
}

# ---------------------------------------------------------------------------
# Argument + env parsing
# ---------------------------------------------------------------------------

parse_arguments() {
  SCENARIO="${1:-}"
  NAMESPACE="${2:-telark}"

  if [[ -z "${SCENARIO}" ]]; then
    die "usage: $(basename "$0") <${VALID_SCENARIOS[*]}> [namespace]"
  fi
  if ! is_valid_scenario "${SCENARIO}"; then
    die "unknown scenario '${SCENARIO}'. Valid: ${VALID_SCENARIOS[*]}"
  fi
}

apply_env_defaults() {
  BASE_URL="${BASE_URL:-${DEFAULT_BASE_URL}}"
  BASELINE_RPS="${BASELINE_RPS:-${DEFAULT_BASELINE_RPS}}"
  MAX_RPS="${MAX_RPS:-${DEFAULT_MAX_RPS}}"
  SEED_POOL_SIZE="${SEED_POOL_SIZE:-${DEFAULT_SEED_POOL_SIZE}}"
  WAIT_TIMEOUT="${WAIT_TIMEOUT:-${DEFAULT_WAIT_TIMEOUT}}"
  IMAGE="${IMAGE:-${DEFAULT_IMAGE}}"
}

compute_run_identifiers() {
  RUN_TAG="${SCENARIO}-$(date +%Y%m%d-%H%M%S)"
  JOB_NAME="k6-${RUN_TAG}"
  CONFIGMAP_NAME="k6-scripts-${RUN_TAG}"
  SEED_PREFIX="${SEED_PREFIX:-k6-${RUN_TAG}}"
}

compute_paths() {
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  K6_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
  JOB_TEMPLATE="${SCRIPT_DIR}/job.yaml"
  RESULTS_DIR="${K6_DIR}/results"
  RUN_LOG="${RESULTS_DIR}/${RUN_TAG}.log"
  RUN_JSON_DIR="${RESULTS_DIR}/${RUN_TAG}-json"

  FLAT_DIR="$(mktemp -d -t k6-flat-XXXXXX)"
  RENDERED_JOB="$(mktemp -t k6-job-XXXXXX.yaml)"

  mkdir -p "${RESULTS_DIR}"
}

# ---------------------------------------------------------------------------
# Cleanup (registered with trap; runs on EXIT/INT/TERM)
# ---------------------------------------------------------------------------

cleanup_all() {
  echo
  log cleanup "deleting job ${NAMESPACE}/${JOB_NAME}"
  kubectl -n "${NAMESPACE}" delete job "${JOB_NAME}" \
    --ignore-not-found --wait=false >/dev/null 2>&1 || true

  log cleanup "deleting configmap ${NAMESPACE}/${CONFIGMAP_NAME}"
  kubectl -n "${NAMESPACE}" delete configmap "${CONFIGMAP_NAME}" \
    --ignore-not-found --wait=false >/dev/null 2>&1 || true

  rm -rf "${FLAT_DIR}" "${RENDERED_JOB}" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Script flattening + ConfigMap upload
# ---------------------------------------------------------------------------

flatten_k6_scripts() {
  log build "flattening k6 scripts into ${FLAT_DIR}"
  cp "${K6_DIR}"/config/*.js "${FLAT_DIR}/"
  cp "${K6_DIR}"/lib/*.js    "${FLAT_DIR}/"
  cp "${K6_DIR}"/scenarios/*.js "${FLAT_DIR}/"

  # ConfigMap volume mounts all keys in one directory. Rewrite relative imports.
  sed_inplace -E "s|'\\.\\./config/|'./|g; s|'\\.\\./lib/|'./|g" "${FLAT_DIR}"/*.js

  if grep -nE "from '\\.\\." "${FLAT_DIR}"/*.js >/dev/null; then
    grep -nE "from '\\.\\." "${FLAT_DIR}"/*.js >&2
    die "relative parent imports remain after flatten"
  fi
}

build_configmap_arguments() {
  CM_FILE_ARGS=()
  for f in "${FLAT_DIR}"/*.js; do
    CM_FILE_ARGS+=("--from-file=$(basename "$f")=$f")
  done
}

create_configmap() {
  build_configmap_arguments
  local file_count
  file_count=$(ls "${FLAT_DIR}" | wc -l | tr -d ' ')
  log apply "configmap ${NAMESPACE}/${CONFIGMAP_NAME} (${file_count} files)"
  kubectl -n "${NAMESPACE}" create configmap "${CONFIGMAP_NAME}" \
    "${CM_FILE_ARGS[@]}" >/dev/null
}

# ---------------------------------------------------------------------------
# Job manifest rendering + apply
# ---------------------------------------------------------------------------

render_job_manifest() {
  sed \
    -e "s|__RUN__|${RUN_TAG}|g" \
    -e "s|__SCENARIO__|${SCENARIO}|g" \
    -e "s|__BASE_URL__|${BASE_URL}|g" \
    -e "s|__BASELINE_RPS__|${BASELINE_RPS}|g" \
    -e "s|__MAX_RPS__|${MAX_RPS}|g" \
    -e "s|__SEED_POOL_SIZE__|${SEED_POOL_SIZE}|g" \
    -e "s|__SEED_PREFIX__|${SEED_PREFIX}|g" \
    -e "s|__CONFIGMAP__|${CONFIGMAP_NAME}|g" \
    "${JOB_TEMPLATE}" > "${RENDERED_JOB}"

  if [[ "${IMAGE}" != "${DEFAULT_IMAGE}" ]]; then
    sed_inplace -E "s|image: ${DEFAULT_IMAGE}|image: ${IMAGE}|" "${RENDERED_JOB}"
  fi
}

print_run_banner() {
  log apply "scenario=${SCENARIO} run=${RUN_TAG} ns=${NAMESPACE}"
  echo "       BASE_URL=${BASE_URL}"
  echo "       BASELINE_RPS=${BASELINE_RPS} MAX_RPS=${MAX_RPS} SEED_POOL_SIZE=${SEED_POOL_SIZE}"
  echo "       SEED_PREFIX=${SEED_PREFIX}"
}

apply_job_manifest() {
  kubectl apply -f "${RENDERED_JOB}"
}

# ---------------------------------------------------------------------------
# Job execution: wait for pod, stream logs, wait for completion
# ---------------------------------------------------------------------------

wait_for_pod_to_appear() {
  log wait "pod scheduling (up to ${POD_APPEAR_TIMEOUT_SECONDS}s)"
  POD_NAME=""
  for _ in $(seq 1 "${POD_APPEAR_TIMEOUT_SECONDS}"); do
    POD_NAME=$(kubectl -n "${NAMESPACE}" get pod -l "run=${RUN_TAG}" \
      -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
    if [[ -n "${POD_NAME}" ]]; then return 0; fi
    sleep 1
  done

  kubectl -n "${NAMESPACE}" describe job "${JOB_NAME}" || true
  die "pod for run=${RUN_TAG} did not appear within ${POD_APPEAR_TIMEOUT_SECONDS}s"
}

wait_for_pod_to_start_running() {
  log wait "pod ${POD_NAME} container start (up to 120s)"
  local phase=""
  for _ in $(seq 1 120); do
    phase=$(kubectl -n "${NAMESPACE}" get pod "${POD_NAME}" \
      -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    case "${phase}" in
      Running|Succeeded|Failed) return 0 ;;
    esac
    sleep 1
  done

  kubectl -n "${NAMESPACE}" describe pod "${POD_NAME}" || true
  die "pod ${POD_NAME} stuck in phase '${phase}' after 120s"
}

stream_pod_logs() {
  log logs "streaming to ${RUN_LOG}"
  echo
  kubectl -n "${NAMESPACE}" logs --follow \
    --pod-running-timeout="${POD_RUNNING_TIMEOUT}" \
    "${POD_NAME}" 2>&1 | tee "${RUN_LOG}" || true
}

wait_for_job_completion() {
  echo
  log wait "job completion (timeout ${WAIT_TIMEOUT})"

  if kubectl -n "${NAMESPACE}" wait --for=condition=complete \
       --timeout="${WAIT_TIMEOUT}" "job/${JOB_NAME}" 2>/dev/null; then
    JOB_STATUS="complete"
  elif kubectl -n "${NAMESPACE}" wait --for=condition=failed \
       --timeout=10s "job/${JOB_NAME}" 2>/dev/null; then
    JOB_STATUS="failed"
  else
    JOB_STATUS="unknown"
  fi
  log status "${JOB_STATUS}"
}

# ---------------------------------------------------------------------------
# Results
# ---------------------------------------------------------------------------

copy_results_from_pod() {
  if ! kubectl -n "${NAMESPACE}" get pod "${POD_NAME}" >/dev/null 2>&1; then
    log copy "pod gone before copy — skipping"
    return 0
  fi

  log copy "${POD_NAME}:/tmp/results/. → ${RUN_JSON_DIR}"
  mkdir -p "${RUN_JSON_DIR}"
  # /. suffix copies contents flat (avoids nested results/ dir).
  if ! kubectl -n "${NAMESPACE}" cp "${POD_NAME}:/tmp/results/." \
       "${RUN_JSON_DIR}" 2>/dev/null; then
    echo "       (no JSON files in /tmp/results — handleSummary may have skipped)"
  fi
}

print_done_summary() {
  echo
  log done "scenario=${SCENARIO} run=${RUN_TAG} status=${JOB_STATUS}"
  echo "       log:  ${RUN_LOG}"
  echo "       json: ${RUN_JSON_DIR}/"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  parse_arguments "$@"
  apply_env_defaults
  compute_run_identifiers
  compute_paths

  trap cleanup_all EXIT INT TERM

  flatten_k6_scripts
  create_configmap
  render_job_manifest
  print_run_banner
  apply_job_manifest

  wait_for_pod_to_appear
  log pod "${POD_NAME}"
  wait_for_pod_to_start_running
  stream_pod_logs
  wait_for_job_completion
  copy_results_from_pod
  print_done_summary
}

main "$@"
