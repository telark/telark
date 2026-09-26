"""Central place for all text and constant values used across the local analyzer."""

# -----------------------------------------------------------------------------
# Environment variable names and their defaults (parsed only in config.py)
# -----------------------------------------------------------------------------
ENV_REDIS_HOST = "REDIS_HOST"
ENV_REDIS_PORT = "REDIS_PORT"
ENV_REDIS_URL = "REDIS_URL"
ENV_REDIS_POOL_SIZE = "REDIS_POOL_SIZE"
ENV_OLLAMA_HOST = "OLLAMA_HOST"
ENV_OLLAMA_AUTO_PULL = "OLLAMA_AUTO_PULL"
ENV_LOG_LEVEL = "LOG_LEVEL"
ENV_API_PORT = "API_PORT"
ENV_AUTH_SERVICE_URL = "AUTH_SERVICE_URL"
ENV_EXPORTER_SERVICE_URL = "EXPORTER_SERVICE_URL"
ENV_SERVICE_TOKEN = "TELARK_SERVICE_TOKEN"
ENV_ANALYZER_MAX_STEPS = "ANALYZER_MAX_STEPS"
ENV_ANALYZER_MAX_TOOL_CALLS = "ANALYZER_MAX_TOOL_CALLS"
ENV_ANALYZER_TOOL_RESULT_MAX_BYTES = "ANALYZER_TOOL_RESULT_MAX_BYTES"
ENV_ANALYZER_WALL_SEC = "ANALYZER_WALL_SEC"
ENV_ANALYZER_LOOP_TIMEOUT_SEC = "ANALYZER_LOOP_TIMEOUT_SEC"
ENV_ANALYZER_EMIT_TIMEOUT_SEC = "ANALYZER_EMIT_TIMEOUT_SEC"
ENV_ANALYZER_CHARS_PER_TOKEN = "ANALYZER_CHARS_PER_TOKEN"
ENV_ANALYZER_CONTEXT_TOKENS = "ANALYZER_CONTEXT_TOKENS"
ENV_ANALYZER_QUEUE_MAX = "ANALYZER_QUEUE_MAX"
ENV_ANALYZER_AUTO_COOLDOWN_SEC = "ANALYZER_AUTO_COOLDOWN_SEC"
ENV_ANALYZER_MANUAL_COOLDOWN_SEC = "ANALYZER_MANUAL_COOLDOWN_SEC"
ENV_ANALYZER_CONFIG_POLL_SEC = "ANALYZER_CONFIG_POLL_SEC"
ENV_ANALYZER_MODE = "ANALYZER_MODE"
ENV_ANALYZER_NUM_THREAD = "ANALYZER_NUM_THREAD"
ENV_ANALYZER_NARRATE_TIMEOUT_SEC = "ANALYZER_NARRATE_TIMEOUT_SEC"
ENV_ANALYZER_REVIEW_INTERVAL_SEC = "ANALYZER_REVIEW_INTERVAL_SEC"
ENV_ANALYZER_REVIEW_TICK_SEC = "ANALYZER_REVIEW_TICK_SEC"
ENV_ANALYZER_REVIEW_APPS_PER_MIN = "ANALYZER_REVIEW_APPS_PER_MIN"
ENV_ANALYZER_REVIEW_WORKLOADS_MAX = "ANALYZER_REVIEW_WORKLOADS_MAX"
ENV_ANALYZER_USAGE_MIN_SAMPLES = "ANALYZER_USAGE_MIN_SAMPLES"
ENV_ANALYZER_USAGE_MIN_SPAN_SEC = "ANALYZER_USAGE_MIN_SPAN_SEC"
ENV_ANALYZER_CHANGE_VELOCITY_PER_DAY = "ANALYZER_CHANGE_VELOCITY_PER_DAY"
ENV_ANALYZER_CHANGE_RISK_MIN_SPAN_SEC = "ANALYZER_CHANGE_RISK_MIN_SPAN_SEC"
ENV_ANALYZER_PRODUCTION_PATTERN = "ANALYZER_PRODUCTION_PATTERN"
ENV_CORS_ALLOWED_ORIGINS = "CORS_ALLOWED_ORIGINS"

DEFAULT_REDIS_HOST = "localhost"
DEFAULT_REDIS_PORT = "6379"
REDIS_URL_TEMPLATE = "redis://{}:{}"
DEFAULT_REDIS_POOL_SIZE = 10
DEFAULT_OLLAMA_HOST = "http://localhost:11434"
DEFAULT_OLLAMA_AUTO_PULL = True
ENV_TRUE_VALUES = ("1", "true", "yes")
DEFAULT_LOG_LEVEL = "INFO"
DEFAULT_API_PORT = 8080
DEFAULT_AUTH_SERVICE_URL = "http://telark-auth-service:8080"
DEFAULT_EXPORTER_SERVICE_URL = "http://telark-exporter-service:8080"
DEFAULT_SERVICE_TOKEN = ""
DEFAULT_ANALYZER_MAX_STEPS = 8
DEFAULT_ANALYZER_MAX_TOOL_CALLS = 8
DEFAULT_ANALYZER_TOOL_RESULT_MAX_BYTES = 2048
DEFAULT_ANALYZER_WALL_SEC = 480
DEFAULT_ANALYZER_LOOP_TIMEOUT_SEC = 120
DEFAULT_ANALYZER_EMIT_TIMEOUT_SEC = 180
DEFAULT_ANALYZER_CHARS_PER_TOKEN = 3.5
DEFAULT_ANALYZER_CONTEXT_TOKENS = 4096
DEFAULT_ANALYZER_QUEUE_MAX = 100
DEFAULT_ANALYZER_AUTO_COOLDOWN_SEC = 600
DEFAULT_ANALYZER_MANUAL_COOLDOWN_SEC = 60
DEFAULT_ANALYZER_CONFIG_POLL_SEC = 30
# fast: rules + one short narration; deep: the multi-step tool loop (>= 4 vCPU or GPU).
MODE_FAST = "fast"
MODE_DEEP = "deep"
ANALYZER_MODES = (MODE_FAST, MODE_DEEP)
DEFAULT_ANALYZER_MODE = MODE_FAST
DEFAULT_ANALYZER_NUM_THREAD = 2
DEFAULT_ANALYZER_NARRATE_TIMEOUT_SEC = 45
# Recommendations: 0 disables the sweep (an analysis run still reviews its app).
DEFAULT_ANALYZER_REVIEW_INTERVAL_SEC = 7200
DEFAULT_ANALYZER_REVIEW_TICK_SEC = 120
DEFAULT_ANALYZER_REVIEW_APPS_PER_MIN = 20
DEFAULT_ANALYZER_REVIEW_WORKLOADS_MAX = 10
DEFAULT_ANALYZER_USAGE_MIN_SAMPLES = 12
DEFAULT_ANALYZER_USAGE_MIN_SPAN_SEC = 43200
DEFAULT_ANALYZER_CHANGE_VELOCITY_PER_DAY = 20.0
DEFAULT_ANALYZER_CHANGE_RISK_MIN_SPAN_SEC = 259200
DEFAULT_ANALYZER_PRODUCTION_PATTERN = r"(^|[-_.])(prod|production|prd)($|[-_.])"

# -----------------------------------------------------------------------------
# Frozen contract, copied verbatim from the Go sources
# internal/data/insights/constants.go:5-33
# -----------------------------------------------------------------------------
STREAM_JOBS = "insights:jobs"
CONSUMER_GROUP = "analyzer"
STREAM_MAX_LEN = 1000

FIELD_NAMESPACE = "namespace"
FIELD_NAME = "name"
FIELD_TRIGGER = "trigger"
FIELD_GENERATION = "generation"

DOCUMENT_KEY_PREFIX = "analyzer:"
# ZSET: member '<ns>/<name>', score = the document's last write in ms (discovery's Insights index reads it).
INDEX_KEY = "analyzer:index"
INDEX_SCORE_MIN = "-inf"
INFLIGHT_KEY_PREFIX = "analyzer:inflight:"
COOLDOWN_MANUAL_KEY_PREFIX = "analyzer:cooldown:manual:"
COOLDOWN_AUTO_KEY_PREFIX = "analyzer:cooldown:auto:"

EVENT_ANALYSIS_QUEUED = "analysis.queued"
EVENT_ANALYSIS_STARTED = "analysis.started"
EVENT_ANALYSIS_FAILED = "analysis.failed"
EVENT_ANALYSIS_FINISHED = "analysis.finished"
EVENT_INSIGHT_CREATED = "insight.created"
EVENT_INSIGHT_UPDATED = "insight.updated"
EVENT_INSIGHT_RESOLVED = "insight.resolved"
EVENT_REVIEW_FINISHED = "review.finished"
EVENT_RUNTIME_CHANGED = "runtime.changed"
EVENT_RUNTIME_PULL = "runtime.pull"
EVENT_RESYNC = "resync"

# Must stay identical to spec.ai.model's pattern in the GlobalConfig CRD.
MODEL_NAME_PATTERN = r"^[a-z0-9][a-z0-9._-]*(:[a-z0-9._-]+)?$"

# internal/data/insights/types.go:3-14
TRIGGER_MANUAL = "manual"
TRIGGER_INCIDENT = "incident"
TRIGGER_RECOVERY = "recovery"

RUNTIME_STATE_ABSENT = "absent"
RUNTIME_STATE_UNREACHABLE = "unreachable"
RUNTIME_STATE_MODEL_MISSING = "model_missing"
RUNTIME_STATE_PULLING = "pulling"
RUNTIME_STATE_UNSUPPORTED = "unsupported"
RUNTIME_STATE_READY = "ready"

# internal/data/resources/application/insights.go (constants block)
INSIGHT_KIND_CRASHLOOP = "crashloop"
INSIGHT_KIND_OOM = "oom"
INSIGHT_KIND_IMAGE_PULL = "image_pull"
INSIGHT_KIND_PROBE_FAILURE = "probe_failure"
INSIGHT_KIND_SCHEDULING = "scheduling"
INSIGHT_KIND_ROLLOUT_STUCK = "rollout_stuck"
INSIGHT_KIND_CONFIG_CHANGE_REGRESSION = "config_change_regression"
INSIGHT_KIND_RESOURCE_PRESSURE = "resource_pressure"
INSIGHT_KIND_OTHER = "other"
# The kinds that stand for a replica shortfall itself, which a paused rollout makes intentional.
PAUSED_RESOLVES_KINDS = (INSIGHT_KIND_ROLLOUT_STUCK, INSIGHT_KIND_CONFIG_CHANGE_REGRESSION, INSIGHT_KIND_OTHER)

CONFIDENCE_LOW = "low"
CONFIDENCE_MEDIUM = "medium"
CONFIDENCE_HIGH = "high"

INSIGHT_SEVERITY_INFO = "info"
INSIGHT_SEVERITY_WARNING = "warning"
INSIGHT_SEVERITY_CRITICAL = "critical"

INSIGHT_STATUS_OPEN = "open"
INSIGHT_STATUS_UPDATED = "updated"
INSIGHT_STATUS_RESOLVED = "resolved"

RUN_STATUS_QUEUED = "queued"
RUN_STATUS_RUNNING = "running"
RUN_STATUS_DONE = "done"
RUN_STATUS_FAILED = "failed"

EVIDENCE_TYPE_CHANGE = "change"
EVIDENCE_TYPE_SNAPSHOT = "snapshot"
EVIDENCE_TYPE_EVENT = "event"
EVIDENCE_TYPE_WORKLOAD = "workload"
EVIDENCE_TYPE_SPEC = "spec"
EVIDENCE_TYPE_OBJECT = "object"
EVIDENCE_TYPE_METRIC = "metric"
EVIDENCE_TYPE_PLAN = "plan"

INSIGHT_CATEGORY_INCIDENT = "incident"
INSIGHT_CATEGORY_RECOMMENDATION = "recommendation"

TRIAGE_STATE_ACKNOWLEDGED = "acknowledged"
TRIAGE_STATE_DISMISSED = "dismissed"
TRIAGE_ACTION_ACKNOWLEDGE = "acknowledge"
TRIAGE_ACTION_DISMISS = "dismiss"
TRIAGE_ACTION_REOPEN = "reopen"

MAX_INSIGHT_TITLE_LENGTH = 120
MAX_INSIGHT_SUMMARY_LENGTH = 400
MAX_INSIGHTS_PER_RUN = 3
MAX_RECOMMENDATIONS_PER_APP = 40
MAX_INSIGHT_PARAMS = 16
MAX_INSIGHT_PARAM_LENGTH = 120

# internal/data/constants/builtin.go:5
DEFAULT_ANALYZER_MODEL = "granite4:350m"

# '/api/v1/' + internal/rest/endpoints/insights/def.go:7-11
API_V1_PREFIX = "/api/v1/"
ANALYZE_PATH = API_V1_PREFIX + "insights/applications/{namespace}/{name}/analyze"
EVENTS_PATH = API_V1_PREFIX + "insights/events"
RUNTIME_PATH = API_V1_PREFIX + "insights/runtime"
RUNTIME_VALIDATE_PATH = API_V1_PREFIX + "insights/runtime/validate"
RUNTIME_PULL_PATH = API_V1_PREFIX + "insights/runtime/pull"
# internal/rest/endpoints/insights/def.go Triage
TRIAGE_PATH = API_V1_PREFIX + "insights/applications/{namespace}/{name}/insights/{id}/triage"

# The Go response envelope, internal/rest/response/base.go:16,26,29-34
ENVELOPE_STATUS = "status"
ENVELOPE_OPERATION = "operation"
ENVELOPE_MESSAGE = "message"
ENVELOPE_DATA = "data"
OPERATION_SUCCESS = "Success"
OPERATION_ERROR = "Error"
# Error responses carry data {code}; the UI maps the code to its message.
ENVELOPE_CODE = "code"

# -----------------------------------------------------------------------------
# Lifetimes and timeouts
# -----------------------------------------------------------------------------
DOCUMENT_TTL_S = 604800
INFLIGHT_TTL_S = 540
JOB_MAX_AGE_S = 1800
READY_PING_TIMEOUT_S = 2
SHUTDOWN_GRACE_S = 5

# -----------------------------------------------------------------------------
# Insight lifecycle (code-owned; the model only proposes)
# -----------------------------------------------------------------------------
INSIGHT_ID_TEMPLATE = "{namespace}|{name}|{subject}|{workload_namespace}"
# The id before it named the workload's namespace: merge moves such a card to its new id.
LEGACY_INSIGHT_ID_TEMPLATE = "{namespace}|{name}|{subject}"
# A recommendation's id adds the rule and the workload namespace: it never collides with an incident id.
RECOMMENDATION_ID_TEMPLATE = "{namespace}|{name}|{subject}|{reason}|{workload_namespace}"
TRIAGE_ERROR_NOT_FOUND = "insight_not_found"
TRIAGE_ERROR_INVALID = "invalid_triage"
INSIGHT_ID_LENGTH = 16
SUBJECT_SEPARATOR = "/"
RESOLVED_RETENTION_S = 604800
SEVERITY_RANK = {INSIGHT_SEVERITY_INFO: 0, INSIGHT_SEVERITY_WARNING: 1, INSIGHT_SEVERITY_CRITICAL: 2}
MIN_REFS_FOR_CONFIDENCE = 2
MAX_EVIDENCE_PER_INSIGHT = 4
COOLDOWN_VALUE = "1"

# -----------------------------------------------------------------------------
# lastRun.error codes
# -----------------------------------------------------------------------------
RUN_ERROR_RUNTIME_UNREACHABLE = "runtime_unreachable"
RUN_ERROR_MODEL_NOT_INSTALLED = "model_not_installed"
RUN_ERROR_MODEL_UNSUPPORTED = "model_unsupported"
RUN_ERROR_MODEL_TIMEOUT = "model_timeout"
RUN_ERROR_CONTEXT_OVERFLOW = "context_overflow"
RUN_ERROR_INVALID_TOOL_CALLS = "invalid_tool_calls"
RUN_ERROR_INVALID_OUTPUT = "invalid_output"
RUN_ERROR_BUSY = "busy"
RUN_ERROR_APP_NOT_FOUND = "app_not_found"
RUN_ERROR_STORAGE_UNAVAILABLE = "storage_unavailable"
RUN_ERROR_JOB_EXPIRED = "job_expired"
RUN_ERROR_RUN_IN_PROGRESS = "run_in_progress"
RUN_ERROR_JOB_DROPPED = "job_dropped"
RUN_ERROR_INTERNAL = "internal_error"

# -----------------------------------------------------------------------------
# Redis keys and time formats
# -----------------------------------------------------------------------------
RFC3339_FORMAT = "%Y-%m-%dT%H:%M:%SZ"
JSON_COMPACT_SEPARATORS = (",", ":")
STREAM_ID_SEPARATOR = "-"

# -----------------------------------------------------------------------------
# API server
# -----------------------------------------------------------------------------
API_HOST = "0.0.0.0"

# Dev only: behind nginx the UI is same-origin, so the default (none) adds no CORS headers.
DEFAULT_CORS_ALLOWED_ORIGINS = ""
CORS_ORIGINS_SEPARATOR = ","
CORS_ALLOWED_METHODS = ["GET", "POST", "OPTIONS"]
CORS_ALLOWED_HEADERS = ["*"]
CORS_MAX_AGE_S = 600

SERVICE_NAME = "analyzer-service"
STATUS_READY_PATH = "/api/v1/status/ready"
STATUS_LIVE_PATH = "/api/v1/status/live"
STATUS_READY = "ready"
STATUS_NOT_READY = "not_ready"
STATUS_ALIVE = "alive"
PROBE_KEY_STATUS = "status"
PROBE_KEY_SERVICE = "service"

METHOD_GET = "GET"
METHOD_POST = "POST"

# k8s.io/apimachinery validation IsDNS1123Label (max 63 characters).
DNS1123_LABEL_PATTERN = r"^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$"

API_ERROR_INVALID_APP = "invalid_app"
API_ERROR_ANALYZER_DISABLED = "analyzer_disabled"
API_ERROR_COOLDOWN_ACTIVE = "cooldown_active"
API_ERROR_QUEUE_FULL = "queue_full"
API_ERROR_AUTO_PULL_DISABLED = "auto_pull_disabled"
API_ERROR_INVALID_REQUEST = "invalid_request"
API_ERROR_MODEL_NOT_ALLOWED = "model_not_allowed"
API_ERROR_TOO_MANY_STREAMS = "too_many_streams"

# Every request body is a few bytes of JSON; larger ones are refused before FastAPI buffers them.
REQUEST_BODY_MAX_BYTES = 64 * 1024
HEADER_CONTENT_LENGTH = b"content-length"
ASGI_SCOPE_HTTP = "http"
ASGI_MESSAGE_HTTP_REQUEST = "http.request"
ASGI_FIELD_BODY = "body"
MSG_BODY_TOO_LARGE = "request body too large"
# The CRD caps GlobalConfig spec.ai.model at the same length.
MODEL_NAME_MAX_LENGTH = 128

LOG_ANALYZE_FAILED = "analyze: storage unavailable: {}"
LOG_QUEUED_NOT_STAMPED = "analyze: job queued, lastRun not stamped: {}"
LOG_TRIAGE_FAILED = "triage: storage unavailable: {}"

# ?apps= as discovery parses it (handlers/insights/handler.go:63-84): 'ns/name,ns/name'.
APPS_SEPARATOR = ","
APP_KEY_SEPARATOR = "/"

SSE_PING_S = 15
SSE_MEDIA_TYPE = "text/event-stream"
# X-Accel-Buffering: no turns off nginx response buffering for this response only.
SSE_HEADERS = {"Cache-Control": "no-cache", "X-Accel-Buffering": "no"}
SSE_CONNECTED = ": connected\n\n"
SSE_PING = ": ping\n\n"
SSE_EVENT_TEMPLATE = "event: {name}\ndata: {data}\n\n"
# An open stream re-checks its session this often and ends once auth-service answers 401 or 403.
SSE_RECHECK_PINGS = 4
SSE_RECHECK_S = SSE_PING_S * SSE_RECHECK_PINGS
SSE_MAX_STREAMS_PER_USER = 8

# -----------------------------------------------------------------------------
# Exporter (the sole CRD reader), called with the service token
# -----------------------------------------------------------------------------
GLOBALCONFIG_PATH = "/api/v1/resources/globalconfig/get"
APPLICATION_GET_PATH = "/api/v1/resources/applications/{name}/get"
APPLICATIONS_LIST_PATH = "/api/v1/resources/applications/get"
PLANS_LIST_PATH = "/api/v1/plans/protection/get"
PLAN_ENVIRONMENTS_PATH = "/api/v1/classification/categories/scope/plan-environments/get"
PARAM_VIEW = "view"
VIEW_SUMMARY = "summary"
FIELD_ITEMS = "items"
EXPORTER_TIMEOUT_S = 5.0
# The full app list at 2 000 apps takes seconds; the 5 s read timeout would skip every sweep tick.
EXPORTER_LIST_TIMEOUT_S = 30.0
GLOBALCONFIG_FIELD_AI = "ai"
GLOBALCONFIG_FIELD_EXCLUDED_NAMESPACES = "excludedNamespaces"
AI_FIELD_ENABLED = "enabled"
AI_FIELD_MODEL = "model"
AI_FIELD_AUTO_ANALYZE = "autoAnalyze"
LOG_GLOBALCONFIG_FETCH_FAILED = "failed to read the analyzer config from GlobalConfig, keeping the last good value: {}"

# -----------------------------------------------------------------------------
# Ollama runtime (an external runtime: no service token, no envelope)
# -----------------------------------------------------------------------------
OLLAMA_TAGS_PATH = "/api/tags"
OLLAMA_SHOW_PATH = "/api/show"
OLLAMA_CHAT_PATH = "/api/chat"
OLLAMA_PULL_PATH = "/api/pull"
OLLAMA_META_TIMEOUT_S = 5
# Never unload: a reload re-runs Ollama's free-memory check, which counts page cache.
OLLAMA_KEEP_ALIVE = -1
CHAT_TEMPERATURE = 0.2
ROLE_SYSTEM = "system"
ROLE_USER = "user"
ROLE_TOOL = "tool"
TOOL_TYPE_FUNCTION = "function"
CAPABILITY_TOOLS = "tools"
PULL_STATUS_SUCCESS = "success"
OLLAMA_FIELD_MODELS = "models"
OLLAMA_FIELD_NAME = "name"
OLLAMA_FIELD_CAPABILITIES = "capabilities"
OLLAMA_FIELD_ERROR = "error"
# Matched against the lower-cased error body.
OLLAMA_UNSUPPORTED_TOOLS_MARKER = "does not support tools"
OLLAMA_CONTEXT_MARKER = "context"
OLLAMA_OVERFLOW_MARKERS = ("exceed", "too long", "truncat")
OLLAMA_PULL_INCOMPLETE = "pull stream ended without success"

# -----------------------------------------------------------------------------
# Authorization
# -----------------------------------------------------------------------------
AUTHZ_PERMISSIONS_PATH = "/api/v1/auth/permissions"
AUTHZ_TIMEOUT_SECONDS = 5.0

HEADER_SESSION_TOKEN = "X-Session-Token"
HEADER_SERVICE_TOKEN = "X-Service-Token"

SCOPE_ALL = "ALL"
SCOPE_SETTINGS = "settings"
SCOPE_INSIGHTS = "insights"

ROLE_STATUS_ACTIVE = "Active"

# x-ware authz.RuleKey (rule.go:5-7): lower('<scope>.<action>.deny').
RULE_KEY_TEMPLATE = "{scope}.{action}.deny"
# internal/data/resources/role/rules.go
ACTION_CONTROL_AI_INSIGHTS = "controlainsights"
ACTION_ANALYZE_INSIGHTS = "analyzeinsights"
ACTION_TRIAGE_INSIGHTS = "triageinsights"

PERMISSION_LEVEL_READONLY = "ReadOnly"
PERMISSION_LEVEL_CONTRIBUTOR = "Contributor"
PERMISSION_LEVEL_OWNER = "Owner"
PERMISSION_LEVEL_ADMIN = "Admin"

# Unknown levels rank 0 so they can never grant access.
PERMISSION_RANKS = {
    PERMISSION_LEVEL_READONLY: 1,
    PERMISSION_LEVEL_CONTRIBUTOR: 2,
    PERMISSION_LEVEL_OWNER: 3,
    PERMISSION_LEVEL_ADMIN: 4,
}

# auth-service permissions response (auth/internal/handlers/authorisation/types.go)
PERMISSIONS_FIELD_USER_ID = "userID"
PERMISSIONS_FIELD_ROLES = "roles"

MSG_AUTHZ_MISSING_SESSION = "a session token is required"
MSG_AUTHZ_INVALID_SESSION = "the session is invalid or has expired"
MSG_AUTHZ_FORBIDDEN = "you do not have permission to perform this action"
MSG_AUTHZ_UNAVAILABLE = "unable to verify permissions"

LOG_AUTHZ_RESOLVE_FAILED = "authz: failed to resolve permissions: {}"

# -----------------------------------------------------------------------------
# Tools (read-only; arguments are validated by the pydantic models in models.py)
# -----------------------------------------------------------------------------
TOOL_GET_APP_OVERVIEW = "get_app_overview"
TOOL_GET_CHANGE_HISTORY = "get_change_history"
TOOL_GET_WORKLOAD_STATUS = "get_workload_status"
TOOL_GET_RECENT_EVENTS = "get_recent_events"

TOOL_DESCRIPTIONS = {
    TOOL_GET_APP_OVERVIEW: (
        "Health, namespaces, workloads, drift, last change and recent snapshot ids of the application."
    ),
    TOOL_GET_CHANGE_HISTORY: "Recorded configuration changes of the application, newest first.",
    TOOL_GET_WORKLOAD_STATUS: (
        "Live rollout conditions, replica counts, most-restarted pods and images of one workload "
        "listed by get_app_overview; give its namespace when two listed workloads share a kind and name."
    ),
    TOOL_GET_RECENT_EVENTS: "Recent Kubernetes events of the application's workloads and their pods.",
}

# Per-run caps. The overview is pure and cheap, so a repeat is served, never capped.
TOOL_CAPS = {
    TOOL_GET_APP_OVERVIEW: 1,
    TOOL_GET_CHANGE_HISTORY: 2,
    TOOL_GET_WORKLOAD_STATUS: 3,
    TOOL_GET_RECENT_EVENTS: 2,
}
TOOL_TIMEOUT_S = 5
# Above TOOL_TIMEOUT_S so the per-call asyncio.timeout fires first.
K8S_HTTP_TIMEOUT_S = 10

TOOL_ERROR_UNKNOWN_TOOL = "unknown_tool"
TOOL_ERROR_INVALID_ARGUMENTS = "invalid_arguments"
TOOL_ERROR_CALL_CAP_EXCEEDED = "call_cap_exceeded"
TOOL_ERROR_UNKNOWN_WORKLOAD = "unknown_workload"
TOOL_ERROR_UNKNOWN_NAMESPACE = "unknown_namespace"
TOOL_ERROR_TIMEOUT = "timeout"
TOOL_ERROR_K8S_UNAVAILABLE = "k8s_unavailable"
TOOL_ERROR_K8S_ERROR = "k8s_error"
TOOL_FIELD_ERROR = "error"
TOOL_FIELD_DETAIL = "detail"

REF_WORKLOAD = "workload:{kind}/{name}"
REF_SNAPSHOT = "snap:{id}"
REF_GENERATION = "gen:{generation}"
REF_EVENT = "{reason}@{object}@{last}"
SUBJECT_TEMPLATE = "{kind}/{name}"

WORKLOAD_DEPLOYMENT = "deployment"
WORKLOAD_STATEFULSET = "statefulset"
WORKLOAD_DAEMONSET = "daemonset"
# lowercased kind -> (API kind, API resource)
WORKLOAD_KINDS = {
    WORKLOAD_DEPLOYMENT: ("Deployment", "deployments"),
    WORKLOAD_STATEFULSET: ("StatefulSet", "statefulsets"),
    WORKLOAD_DAEMONSET: ("DaemonSet", "daemonsets"),
}
K8S_KIND_REPLICASET = "ReplicaSet"
K8S_KIND_POD = "Pod"
POD_PHASE_PENDING = "Pending"
POD_PHASE_RUNNING = "Running"
CONDITION_DISRUPTION_TARGET = "DisruptionTarget"
RESOURCE_CPU = "cpu"
RESOURCE_MEMORY = "memory"

# Generated-name suffixes, k8s.io/apimachinery/pkg/util/rand/rand.go:83 (SafeEncodeString).
SAFE_ENCODE_ALPHABET = "bcdfghjklmnpqrstvwxz2456789"
_SAFE = "[" + SAFE_ENCODE_ALPHABET + "]"
REPLICASET_NAME_PATTERN = "^{name}-" + _SAFE + "{{1,10}}$"
DEPLOYMENT_POD_NAME_PATTERN = "^{name}-" + _SAFE + "{{1,10}}-" + _SAFE + "{{5}}$"
DAEMONSET_POD_NAME_PATTERN = "^{name}-" + _SAFE + "{{5}}$"
STATEFULSET_POD_NAME_PATTERN = r"^{name}-\d+$"

# -----------------------------------------------------------------------------
# Kubernetes helpers (helpers.py): quantities, selectors, image references, probes
# -----------------------------------------------------------------------------
QUANTITY_PATTERN = r"([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)([a-zA-Z]*)"
QUANTITY_EPSILON = 1e-9
CPU_MILLI_PER_CORE = 1000
# suffix -> millicores per unit; '' = cores (kcore writes '%.2f' cores or '%dm').
CPU_MILLI_SUFFIXES = {"": 1000.0, "m": 1.0, "u": 1e-3, "n": 1e-6, "k": 1e6}
# suffix -> bytes per unit; 'B' is kcore's plain-bytes suffix.
MEM_BYTE_SUFFIXES = {
    "": 1.0, "B": 1.0, "k": 1e3, "M": 1e6, "G": 1e9, "T": 1e12, "P": 1e15, "E": 1e18,
    "Ki": 2.0**10, "Mi": 2.0**20, "Gi": 2.0**30, "Ti": 2.0**40, "Pi": 2.0**50, "Ei": 2.0**60, "m": 1e-3,
}
MEM_FORMAT_UNITS = (("Gi", 2**30), ("Mi", 2**20), ("Ki", 2**10), ("B", 1))
# (label present, label value, values) -> matches
SELECTOR_OPERATORS = {
    "In": lambda present, value, values: present and value in values,
    "NotIn": lambda present, value, values: not present or value not in values,
    "Exists": lambda present, value, values: present,
    "DoesNotExist": lambda present, value, values: not present,
}
# distribution/reference: lower-case path components, optional registry host[:port], tag, digest.
IMAGE_REFERENCE_PATTERN = (
    r"(?:[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?(?::\d+)?/)?"
    r"[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*"
    r"(?::[\w][\w.-]{0,127})?(?:@[a-z0-9]+:[a-fA-F0-9]{32,})?"
)
DEFAULT_REGISTRY = "docker.io"
DEFAULT_REPOSITORY_PREFIX = "library/"
DOMAIN_MARKERS = (".", ":")
LOCALHOST = "localhost"
TAG_SEPARATOR = ":"
DIGEST_SEPARATOR = "@"
PROBE_DEFAULT_FAILURE_THRESHOLD = 3
PROBE_DEFAULT_PERIOD_S = 10
# handler -> the fields that name what it checks
PROBE_HANDLER_FIELDS = {
    "httpGet": ("path", "port", "host", "scheme"),
    "tcpSocket": ("port", "host"),
    "grpc": ("port", "service"),
    "exec": ("command",),
}

# -----------------------------------------------------------------------------
# In-cluster API (GET only, the pod's service-account token and CA)
# -----------------------------------------------------------------------------
SA_TOKEN_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/token"
SA_CA_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
K8S_API_BASE = "https://kubernetes.default.svc"
K8S_WORKLOAD_PATH = "/apis/apps/v1/namespaces/{namespace}/{resource}/{name}"
K8S_PODS_PATH = "/api/v1/namespaces/{namespace}/pods"
K8S_EVENTS_PATH = "/api/v1/namespaces/{namespace}/events"
HEADER_AUTHORIZATION = "Authorization"
BEARER_TEMPLATE = "Bearer {}"
K8S_PARAM_LABEL_SELECTOR = "labelSelector"
K8S_PARAM_FIELD_SELECTOR = "fieldSelector"
K8S_PARAM_LIMIT = "limit"
WARNING_FIELD_SELECTOR = "type=Warning"

EVENTS_LIST_LIMIT = 200
EVENTS_KEEP_MAX = 12
EVENT_MESSAGE_MAX = 160
EVENT_DEDUPE_CHARS = 120
CHANGE_VALUE_MAX = 80
CHANGES_PER_ENTRY_MAX = 6
EVENT_NAMESPACES_MAX = 5
PODS_MAX = 5
SNAPSHOT_IDS_MAX = 3

# -----------------------------------------------------------------------------
# Analysis loop budget (design Caps: the per-request fit check and the wall reserve)
# -----------------------------------------------------------------------------
NUM_PREDICT_LOOP = 400
NUM_PREDICT_EMIT = 1024
PER_STEP_GROWTH = 735
EMIT_USER_TOKENS = 100
# Loop timeout + EMIT timeout: below this much wall left, a loop step would starve the EMIT.
EMIT_RESERVE_S = 300
EMIT_ATTEMPTS = 2
BUSY_SLEEP_S = 15
MAX_TOOL_CALLS_PER_STEP = 2
MAX_CONSECUTIVE_STRIKES = 2
# A timeout, k8s_error or k8s_unavailable is the cluster's fault, never a strike.
STRIKE_TOOL_ERRORS = (
    TOOL_ERROR_INVALID_ARGUMENTS,
    TOOL_ERROR_UNKNOWN_TOOL,
    TOOL_ERROR_UNKNOWN_WORKLOAD,
    TOOL_ERROR_UNKNOWN_NAMESPACE,
)

# -----------------------------------------------------------------------------
# Fast mode: rules (rules.py) decide every insight from the gathered tool results
# -----------------------------------------------------------------------------
FAST_STATUS_MAX = 3
FAST_HISTORY_LIMIT = 5
FAST_EVENTS_SINCE_MIN = 60
CHANGE_CORRELATION_WINDOW_S = 1800
S_PER_MINUTE = 60
# The discovery change classes that alter what runs: an image rollout, config or resource limits. Scaling,
# topology, drift and the initial snapshot are never cited as a cause.
CORRELATED_CHANGE_CLASSES = ("config", "deployment", "resources")
# Matched as given (Kubernetes reasons) or against the lower-cased event message (markers).
REASON_OOM_KILLED = "OOMKilled"
OOM_EVENT_REASONS = ("OOMKilling", REASON_OOM_KILLED)
IMAGE_PULL_WAITING_REASONS = ("ImagePullBackOff", "ErrImagePull", "InvalidImageName")
REASON_FAILED = "Failed"
REASON_BACKOFF = "BackOff"
IMAGE_PULL_MARKERS = ("pull", "image")
REASON_CRASHLOOP = "CrashLoopBackOff"
CRASHLOOP_MARKER = "restarting failed container"
REASON_UNHEALTHY = "Unhealthy"
PROBE_NAMES = ("liveness", "readiness", "startup")
REASON_FAILED_SCHEDULING = "FailedScheduling"
REASON_EVICTED = "Evicted"
EVICTION_MARKER = "the node was low on resource"
CONDITION_PROGRESSING = "Progressing"
CONDITION_FALSE = "False"
CONDITION_READY = "Ready"
RECOVERED_MIN_UPTIME_S = 60
NONE_VALUE = "none"
FACT_TEMPLATE = "{}: {}"
FACT_WHAT = "what"
CORRELATION_TEMPLATE = "{minutes} min after change gen {generation}"
CORRELATION_DETAIL_TEMPLATE = ": {field} {old}\u2192{new}"
CORRELATION_SUFFIX_TEMPLATE = " It began {}."
CHANGE_PARAM_TEMPLATE = "{field} {old}\u2192{new}"

# -----------------------------------------------------------------------------
# Incident messages (messages.py): sub-reason, params, server title/summary
# -----------------------------------------------------------------------------
REASON_SEPARATOR = "."
REASON_FAILED_CREATE = "FailedCreate"
REASON_PREEMPTED = "Preempted"
REASON_PREEMPTION_BY_SCHEDULER = "PreemptionByScheduler"
REASON_START_ERROR = "StartError"
REASON_PROGRESS_DEADLINE = "ProgressDeadlineExceeded"
REASON_INVALID_IMAGE_NAME = "InvalidImageName"
REASON_CREATE_CONTAINER_CONFIG_ERROR = "CreateContainerConfigError"
CREATE_CONTAINER_ERROR_REASONS = ("CreateContainerError", "RunContainerError")
VOLUME_EVENT_REASONS = ("FailedMount", "FailedAttachVolume")
PROBE_LIVENESS = "liveness"
PROBE_READINESS = "readiness"
PROBE_STARTUP = "startup"
RESTARTING_PROBES = (PROBE_LIVENESS, PROBE_STARTUP)
# kubelet: '<Probe> probe failed: …' or, for exec timeouts and runtime errors, '<Probe> probe errored: …'.
PROBE_MESSAGE_PATTERN = r"^\s*(readiness|liveness|startup) probe (?:failed|errored)\s*:?\s*"
IMAGE_IN_MESSAGE_PATTERN = r'image(?: tag| name)? "([^"]+)"'
# containerd chains 'Failed to pull image "X": … reference "X": <cause>': the cause follows the last '"X": '.
PULL_CAUSE_TEMPLATE = '"{}": '
RPC_ERROR_PREFIX_PATTERN = r"^\s*rpc error: code = \w+ desc = "
# Checked in this order; a marker is a regex matched against the lower-cased message with the image name removed.
# denied_or_missing before unauthorized: Docker Hub answers a missing and a private repository the same way.
IMAGE_PULL_MARKERS_BY_REASON = (
    ("image_pull.invalid_name", (r"invalid reference format",)),
    ("image_pull.denied_or_missing", (r"pull access denied", r"insufficient_scope", r"repository does not exist")),
    ("image_pull.unauthorized", (r"unauthorized", r"authentication required", r"\b(?:401|403)\b")),
    ("image_pull.rate_limited", (r"toomanyrequests", r"\b429\b", r"rate limit")),
    ("image_pull.registry_unreachable", (r"no such host", r"i/o timeout", r"connection refused",
                                         r"network is unreachable", r"tls handshake timeout",
                                         r"context deadline exceeded")),
    ("image_pull.not_found", (r"not found", r"manifest unknown")),
)
EXIT_CODE_REASONS = {
    0: "crashloop.exit_0", 1: "crashloop.exit_1", 126: "crashloop.exit_126", 127: "crashloop.exit_127",
    137: "crashloop.exit_137", 139: "crashloop.exit_139", 143: "crashloop.exit_143",
}
# (failure, marker regexes on the lower-cased probe message), checked in order; 'other' when none matches.
PROBE_FAILURE_MARKERS = (
    ("refused", (r"connection refused",)),
    ("http_status", (r"statuscode: \d+",)),
    ("timeout", (r"timeout", r"timed out", r"context deadline exceeded")),
    ("command", (r"command\b.*\b(?:exit code|exited)",)),
)
PROBE_FAILURE_OTHER = "other"
PROBE_FAILURE_COMMAND = "command"
PROBE_PORT_PATTERN = r":(\d{1,5})\b"
PROBE_STATUS_PATTERN = r"statuscode: (\d+)"
PROBE_TIMEOUT_PATTERN = r"timeout:? (\d+)s"
# failure -> the failureText variants, the first whose placeholders are all params wins.
PROBE_FAILURE_TEXT = {
    "refused": ("connection refused on port {port}", "connection refused"),
    "http_status": ("the endpoint returned HTTP {status}",),
    "timeout": ("no answer within {timeout}s", "no answer in time"),
    "command": ("the check command failed",),
    "other": ("{detail}", "the check failed"),
}
SCHEDULING_AVAILABLE_MARKER = "available:"
SCHEDULING_PREEMPTION_MARKER = "preemption:"
# Dynamic-resource-allocation note the scheduler appends before the preemption part (k8s ≥ 1.34).
SCHEDULING_DRA_NOISE = r"\s*no new claims to deallocate,?"
SCHEDULING_PIECE_SPLIT = r",\s*|\.\s+"
SCHEDULING_COUNT_PATTERN = r"^\s*(\d+)\s"
# (reason, regexes on one lower-cased scheduler piece), in table order: ties go to the earlier reason.
SCHEDULING_SEGMENTS = (
    ("scheduling.insufficient_cpu", (r"insufficient cpu",)),
    ("scheduling.insufficient_memory", (r"insufficient memory",)),
    ("scheduling.taints", (r"untolerated taint", r"had taint")),
    ("scheduling.node_affinity", (r"didn't match pod's node affinity/selector",)),
    ("scheduling.pod_anti_affinity", (r"anti-affinity rules", r"pod affinity rules")),
    ("scheduling.topology_spread", (r"didn't match pod topology spread constraints",)),
    ("scheduling.volume", (r"unbound immediate persistentvolumeclaims", r"volume node affinity conflict",
                           r"persistentvolumeclaim .* not found")),
    ("scheduling.too_many_pods", (r"too many pods",)),
    ("scheduling.host_ports", (r"didn't have free ports",)),
)
SCHEDULING_OTHER = "scheduling.other"
# (reason, regexes on the lower-cased eviction message), in order: node disk pressure ('low on resource:
# ephemeral-storage') is checked before the pod's own local-storage use.
EVICTION_MARKERS = (
    ("resource_pressure.evicted_memory", (r"low on resource: memory",)),
    ("resource_pressure.evicted_disk", (r"nodefs", r"imagefs", r"low on resource: ephemeral-storage")),
    ("resource_pressure.evicted_ephemeral", (r"ephemeral", r"emptydir")),
    ("resource_pressure.evicted_pid", (r"\bpids?\b",)),
)
EVICTION_OTHER = "resource_pressure.other"
QUOTA_MARKERS = (r"exceeded quota",)
# Event aggregation prefix, and the FailedCreate head naming a pod that was never created.
COMBINED_EVENTS_PREFIX = "(combined from similar events): "
FAILED_CREATE_HEAD = r'^Error creating: (?:\w+ "[^"]*" is forbidden: )?'
ADMISSION_MARKERS = (r"denied the request", r"admission webhook")
# changeLog field -> reason (discovery changes/descriptions.go); requests*/limits* by prefix.
CHANGE_FIELD_REASONS = {
    "image": "config_change_regression.image", "chart.version": "config_change_regression.image",
    "envVarKey": "config_change_regression.config", "configMapRef": "config_change_regression.config",
    "secretRef": "config_change_regression.config", "port": "config_change_regression.config",
    "serviceMapping": "config_change_regression.config", "ingressRule": "config_change_regression.config",
}
CHANGE_FIELD_RESOURCE_PREFIXES = ("requests", "limits")
# Discovery's synthetic field on an incident entry: the symptom, never the cause.
CHANGE_FIELD_HEALTH = "health"
CHANGE_REASON_RESOURCES = "config_change_regression.resources"
CHANGE_REASON_OTHER = "config_change_regression.other"
WARNING_REASONS_MAX = 3
WARNING_REASONS_SEPARATOR = ", "
# Every param key an incident may carry: values are cut to MAX_INSIGHT_PARAM_LENGTH, at most MAX_INSIGHT_PARAMS.
INCIDENT_PARAM_KEYS = (
    "workload", "namespace", "pod", "container", "image", "registry", "exitCode", "lastReason", "restarts",
    "ready", "desired", "pending", "message", "probe", "failure", "port", "status", "timeout", "count", "limit",
    "updated", "generation", "change", "warnings", "reasons",
)
PARAM_ELLIPSIS = "\u2026"
# Derived placeholder -> (singular, plural), chosen by the restarts count.
RESTART_PLURALS = {"restartsNoun": ("restart", "restarts"), "timesNoun": ("time", "times")}
RESTARTS_SINGULAR = "1"
# title '{workload} {state}: {what}', summary '{impact} {detail}{change}'.
STATE_DOWN = "is down"
STATE_DEGRADED = "is degraded"
TITLE_WITH_STATE = "{workload} {state}: {what}"
TITLE_NO_STATE = "{workload}: {what}"
IMPACT_DOWN = "0 of {desired} replicas are ready, so {workload} is not serving."
IMPACT_PARTIAL = "{ready} of {desired} replicas are ready."
# When a reason's detail needs params this run did not read (no status beyond FAST_STATUS_MAX): event facts only.
INCIDENT_DETAIL_FALLBACKS = ("Pod {pod} reports: {message}.", "{whatSentence}.")
_SCHEDULER_ONLY = ("The scheduler reports: {message}.",)
# reason -> details tried after its own and before the fallbacks.
INCIDENT_DETAIL_VARIANTS = {
    "resource_pressure.preempted": ("Pod {pod} was removed to make room for a higher-priority pod.",),
    **{reason: _SCHEDULER_ONLY for reason in (
        "scheduling.insufficient_cpu", "scheduling.insufficient_memory", "scheduling.taints",
        "scheduling.node_affinity", "scheduling.pod_anti_affinity", "scheduling.topology_spread", "scheduling.volume",
        "scheduling.too_many_pods", "scheduling.host_ports", "scheduling.other")},
}
# reason -> (what, detail): the 52 incident sub-reasons.
INCIDENT_TEXT = {
    "image_pull.not_found": ("image not found",
        "Pod {pod} cannot pull {image}: the registry has no such tag or repository."),
    "image_pull.denied_or_missing": ("image pull denied",
        "Pod {pod} cannot pull {image}: the registry denied access; the repository may not exist or may need "
        "credentials."),
    "image_pull.unauthorized": ("registry login failed",
        "Pod {pod} cannot pull {image}: the registry rejected the credentials."),
    "image_pull.registry_unreachable": ("registry unreachable",
        "Pod {pod} cannot reach the registry {registry} to pull {image}."),
    "image_pull.invalid_name": ("invalid image name",
        "Pod {pod} cannot start: {image} is not a valid image reference."),
    "image_pull.rate_limited": ("registry rate limit",
        "Pod {pod} cannot pull {image}: the registry is rate-limiting pulls."),
    "image_pull.other": ("image pull failing", "Pod {pod} cannot pull {image}: {message}."),
    "crashloop.probe_kill": ("restarted by its {probe} probe",
        "Container {container} in pod {pod} fails its {probe} probe and was restarted {restarts} {timesNoun}: "
        "{failureText}."),
    "crashloop.init_failure": ("init container failing",
        "Init container {container} in pod {pod} fails (exit code {exitCode}), so the main containers never "
        "start."),
    "crashloop.start_error": ("start command cannot run",
        "Container {container} in pod {pod} cannot run its start command (StartError), {restarts} {restartsNoun}."),
    "crashloop.exit_0": ("container exits right after starting",
        "Container {container} in pod {pod} finished with exit code 0 and was restarted {restarts} {timesNoun}."),
    "crashloop.exit_1": ("application error",
        "Container {container} in pod {pod} exits with code 1 (application error), {restarts} {restartsNoun}."),
    "crashloop.exit_126": ("command not executable",
        "Container {container} in pod {pod} exits with code 126: the start command exists but cannot be "
        "executed."),
    "crashloop.exit_127": ("command not found",
        "Container {container} in pod {pod} exits with code 127: the start command was not found in the image."),
    "crashloop.exit_137": ("killed (SIGKILL)",
        "Container {container} in pod {pod} was killed with SIGKILL (exit code 137), {restarts} {restartsNoun}; no "
        "out-of-memory report was recorded."),
    "crashloop.exit_139": ("segmentation fault",
        "Container {container} in pod {pod} crashed with a segmentation fault (exit code 139), {restarts} "
        "{restartsNoun}."),
    "crashloop.exit_143": ("stopped by SIGTERM",
        "Container {container} in pod {pod} was stopped with SIGTERM (exit code 143), {restarts} {restartsNoun}."),
    "crashloop.exit_other": ("crash-looping",
        "Container {container} in pod {pod} exits with code {exitCode} ({lastReason}), {restarts} {restartsNoun}."),
    "oom.limit": ("out of memory",
        "Container {container} in pod {pod} used more than its {limit} memory limit and was killed (OOMKilled), "
        "{restarts} {restartsNoun}."),
    "oom.node": ("node ran out of memory",
        "Pod {pod} was killed because node memory ran out; container {container} has no memory limit."),
    "probe_failure.readiness": ("failing readiness checks",
        "{count} readiness check failures on pod {pod}: {failureText}. Pods that fail readiness receive no "
        "traffic."),
    "probe_failure.liveness": ("failing liveness checks",
        "{count} liveness check failures on pod {pod}: {failureText}. The container will be restarted if they "
        "continue."),
    "probe_failure.startup": ("failing startup checks",
        "{count} startup check failures on pod {pod}: {failureText}."),
    "scheduling.insufficient_cpu": ("not enough CPU on any node",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.insufficient_memory": ("not enough memory on any node",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.taints": ("nodes are tainted", "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.node_affinity": ("no node matches its node rules",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.pod_anti_affinity": ("affinity rules leave no node",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.topology_spread": ("spread rules leave no node",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.volume": ("its volume cannot be bound",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.too_many_pods": ("nodes are full", "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.host_ports": ("host port already in use",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "scheduling.other": ("pods cannot be scheduled",
        "{pending} pod(s) are pending. The scheduler reports: {message}."),
    "resource_pressure.evicted_memory": ("evicted for node memory",
        "Pod {pod} was evicted because its node ran low on memory: {message}."),
    "resource_pressure.evicted_ephemeral": ("evicted for local disk use",
        "Pod {pod} was evicted for local storage use: {message}."),
    "resource_pressure.evicted_disk": ("evicted for node disk",
        "Pod {pod} was evicted because its node ran low on disk: {message}."),
    "resource_pressure.evicted_pid": ("evicted for process count",
        "Pod {pod} was evicted because its node ran out of process IDs: {message}."),
    "resource_pressure.preempted": ("preempted by a higher-priority pod",
        "Pod {pod} was removed to make room for a higher-priority pod: {message}."),
    "resource_pressure.other": ("pods were evicted", "Pod {pod} was evicted: {message}."),
    "rollout_stuck.progress_deadline": ("rollout exceeded its deadline",
        "The rollout stopped making progress: {updated}/{desired} updated, {ready}/{desired} ready."),
    "rollout_stuck.quota_exceeded": ("blocked by a resource quota", "New pods cannot be created: {message}."),
    "rollout_stuck.admission_denied": ("blocked by an admission policy",
        "New pods are rejected at admission: {message}."),
    "rollout_stuck.incomplete": ("rollout incomplete",
        "The rollout is incomplete: {updated}/{desired} updated, {ready}/{desired} ready."),
    "config_change_regression.image": ("degraded after an image change",
        "{ready} of {desired} replicas are ready since change gen {generation}."),
    "config_change_regression.config": ("degraded after a configuration change",
        "{ready} of {desired} replicas are ready since change gen {generation}."),
    "config_change_regression.resources": ("degraded after a resources change",
        "{ready} of {desired} replicas are ready since change gen {generation}."),
    "config_change_regression.other": ("degraded after a change",
        "{ready} of {desired} replicas are ready since change gen {generation}."),
    "other.container_config_error": ("missing configuration",
        "Pod {pod} cannot create container {container}: a referenced ConfigMap, Secret or key does not exist."),
    "other.create_container_error": ("container cannot be created",
        "Pod {pod} cannot create container {container}: {message}."),
    "other.volume_mount": ("a volume cannot be mounted", "Pod {pod} is waiting for a volume: {message}."),
    "other.degraded": ("not all replicas are ready",
        "{ready} of {desired} replicas are ready and no warning was reported."),
    "other.warnings": ("warning events", "{warnings} warning events, most often: {reasons}."),
}

# Narration: one tool-less, schema-bound call that may only rephrase the rule cards.
NARRATE_TITLE_MAX = 80
NARRATE_SUMMARY_MAX = 240
# At the caps one item is ~20 title + ~60 summary + ~12 JSON-syntax tokens.
NARRATE_NUM_PREDICT_BASE = 16
NARRATE_NUM_PREDICT_PER_INSIGHT = 120
NARRATE_REASON_ITEM_COUNT = "item_count"
NARRATE_REASON_UNFAITHFUL = "unfaithful"
FACT_SEPARATOR = ": "
# A small model's prose is kept only if it adds no markup and no name-like token (a hyphenated word with a
# digit: a pod or image name) that the facts do not contain.
NARRATE_MARKUP_PATTERN = r"[{}\[\]]"
NARRATE_NUMBER_PATTERN = r"\d+(?:\.\d+)?"
NARRATE_NAME_TOKEN_PATTERN = r"[\w.:]*\d[\w.:]*(?:-[\w.:]+)+|[\w.:]+(?:-[\w.:]*\d[\w.:]*)+"
# The symptom words of each incident kind: another kind's word the template lacks means a changed cause.
NARRATE_KIND_WORDS = {
    INSIGHT_KIND_OOM: ("out of memory", "oom"),
    INSIGHT_KIND_IMAGE_PULL: ("pull", "registry"),
    INSIGHT_KIND_CRASHLOOP: ("crash",),
    INSIGHT_KIND_SCHEDULING: ("schedul", "pending"),
    INSIGHT_KIND_RESOURCE_PRESSURE: ("evict",),
    INSIGHT_KIND_PROBE_FAILURE: ("probe", "readiness", "liveness"),
    INSIGHT_KIND_ROLLOUT_STUCK: ("rollout", "quota"),
}
# A word match at a word start: 'oom' never matches 'room', 'schedul' matches 'scheduler'.
NARRATE_WORD_PATTERN = r"\b{}"
TIMING_GATHER = "gather"
TIMING_RULES = "rules"

# -----------------------------------------------------------------------------
# Review (review.py gathers read-only, recommendations.py decides, insights.py merges)
# -----------------------------------------------------------------------------
K8S_SERVICES_PATH = "/api/v1/namespaces/{namespace}/services"
K8S_PDB_PATH = "/apis/policy/v1/namespaces/{namespace}/poddisruptionbudgets"
K8S_HPA_PATH = "/apis/autoscaling/v2/namespaces/{namespace}/horizontalpodautoscalers"
K8S_NETPOL_PATH = "/apis/networking.k8s.io/v1/namespaces/{namespace}/networkpolicies"
NAMESPACE_LIST_LIMIT = 500
LIST_CONTINUE_FIELD = "continue"
REVIEW_WALL_S = 20
USAGE_KEY = "analyzer:usage"
REVIEW_KEY = "analyzer:review"
USAGE_SAMPLES_MAX = 48
# Input families: a rule fires, and resolves, only when all of its families are complete.
FAMILY_WORKLOADS = "W"
FAMILY_SERVICES = "S"
FAMILY_PDBS = "P"
FAMILY_HPAS = "H"
FAMILY_NETPOLS = "N"
FAMILY_USAGE = "U"
FAMILY_APP = "A"
FAMILY_DOCUMENT = "D"
FAMILY_PLANS = "X"
FAMILY_EVENTS = "E"
# family -> the per-namespace list path
NAMESPACE_LISTS = {
    FAMILY_SERVICES: K8S_SERVICES_PATH,
    FAMILY_PDBS: K8S_PDB_PATH,
    FAMILY_HPAS: K8S_HPA_PATH,
    FAMILY_NETPOLS: K8S_NETPOL_PATH,
}
KIND_SERVICE = "service"
SELECTOR_EXPRESSION_TEMPLATES = {
    "In": "{key} in ({values})",
    "NotIn": "{key} notin ({values})",
    "Exists": "{key}",
    "DoesNotExist": "!{key}",
}
SELECTOR_SEPARATOR = ","
USAGE_FIELD_TEMPLATE = "{namespace}/{kind}/{name}"
USAGE_SAMPLE_TIME = "t"
USAGE_SAMPLE_CONTAINERS = "containers"
USAGE_CPU_MILLI = "cpu_m"
USAGE_MEM_BYTES = "mem_b"

# -----------------------------------------------------------------------------
# Recommendations (recommendations.py): the 60 v1 rules and their server texts
# -----------------------------------------------------------------------------
NEAR_LIMIT_RATIO = 0.9
OVERPROVISION_RATIO = 0.3
UNDERPROVISION_RATIO = 1.2
SUSTAINED_LOAD_RATIO = 0.8
OVERPROVISION_MIN_CPU_M = 100
OVERPROVISION_MIN_MEM_B = 128 * 2**20
# A suggested limit leaves ~25% headroom over the p95, a suggested request ~20%; never below p95 x 1.1.
LIMIT_HEADROOM = 1.3
REQUEST_HEADROOM = 1.2
SUGGESTION_FLOOR = 1.1
CPU_STEP_M = 10
MEM_STEP_B = 16 * 2**20
USAGE_PERCENTILE = 0.95
OOM_HISTORY_WINDOW_S = 604800
NO_STARTUP_WINDOW_S = 3600
CHANGE_RATE_WINDOW_S = 604800
CHANGE_RATE_WINDOW_DAYS = 7
ROLLBACKS_WINDOW_S = 604800
FREQUENT_ROLLBACKS_MIN = 2
SHORT_GRACE_MAX_S = 1
RECOMMENDATION_WORKLOAD_KINDS = ("deployment", "statefulset")
HOSTNAME_TOPOLOGY_KEY = "kubernetes.io/hostname"
STRATEGY_RECREATE = "Recreate"
STRATEGY_ROLLING_UPDATE = "RollingUpdate"
DEFAULT_MAX_UNAVAILABLE = "25%"
PERCENT_SUFFIX = "%"
FULL_PERCENT = 100
SERVICE_TYPE_EXTERNAL_NAME = "ExternalName"
DEFAULT_SERVICE_ACCOUNT = "default"
HPA_METRIC_RESOURCE = "Resource"
HPA_TARGET_UTILIZATION = "Utilization"
CONDITION_SCALING_LIMITED = "ScalingLimited"
CONDITION_TRUE = "True"
REASON_TOO_MANY_REPLICAS = "TooManyReplicas"
IMAGE_TAG_LATEST = "latest"
MOVING_TAG_POLICIES = ("IfNotPresent", "Never")
DANGEROUS_CAPABILITIES = ("ALL", "SYS_ADMIN", "NET_ADMIN", "SYS_PTRACE", "SYS_MODULE", "DAC_READ_SEARCH")
CAPABILITY_PREFIX = "CAP_"
HOST_NAMESPACE_FIELDS = (("hostNetwork", "network"), ("hostPID", "PID"), ("hostIPC", "IPC"))
SECRET_NAME_SEGMENTS = ("PASSWORD", "PASSWD", "PWD", "SECRET", "TOKEN", "APIKEY", "CREDENTIALS")
SECRET_NAME_SUFFIXES = ("API_KEY", "PRIVATE_KEY", "ACCESS_KEY")
SECRET_NAME_SKIP_SUFFIXES = (
    "_FILE", "_PATH", "_DIR", "_URL", "_TTL", "_TIMEOUT", "_LENGTH", "_NAME", "_REF", "_ENDPOINT", "_HOST", "_PORT",
    "_ISSUER", "_AUDIENCE", "_EXPIRY", "_HEADER", "_MODE", "_TYPE", "_ENABLED",
)
ENV_NAME_SPLIT_PATTERN = r"[_-]"
ENV_REFERENCE_PATTERN = r"\$\([A-Za-z_][A-Za-z0-9_]*\)"
ENV_SCALAR_PATTERN = r"[+-]?\d+(\.\d+)?|true|false|yes|no|on|off"
ENV_VARS_MAX = 5
ENV_FROM_TEMPLATE = "envFrom {}"
PLAN_PHASE_ACTIVE = "active"
PLAN_PHASE_SCHEDULED = "scheduled"
PLAN_MODE_AUDIT = "audit"
PLAN_SCOPE_APPLICATIONS = "applications"
PLAN_SCOPE_NAMESPACES = "namespaces"
ROOT_MODE_EXPLICIT = "explicit"
ROOT_MODE_UNVERIFIED = "unverified"
ROOT_MODE_TEXT = {
    ROOT_MODE_EXPLICIT: "runs as root (runAsUser 0)",
    ROOT_MODE_UNVERIFIED: "does not enforce a non-root user; it runs as root if the image does",
}
LIST_SEPARATOR = ", "
AND_SEPARATOR = " and "
HOST_NAMESPACES_TEMPLATE = "{} namespace"
HOST_NAMESPACES_PLURAL_TEMPLATE = "{} namespaces"
SELECTOR_PAIR_TEMPLATE = "{}={}"
VELOCITY_FORMAT = "{:.1f}"
DATE_FORMAT = "%Y-%m-%d"
KEY_TEMPLATE = "{subject}#{container}/{resource}"
SUBJECT_SERVICE_TEMPLATE = "service/{name}"
SUBJECT_APPLICATION_TEMPLATE = "application/{name}"
REF_SPEC = "spec:{subject}#{path}"
REF_OBJECT = "object:{kind}/{name}"
REF_METRIC = "metric:{subject}#{container}/{resource}.p95"
REF_METRIC_WORKLOAD = "metric:{subject}#{resource}.p95"
REF_PLAN = "plan:{id}"
K8S_KIND_SERVICE = "Service"
K8S_KIND_PDB = "PodDisruptionBudget"
K8S_KIND_HPA = "HorizontalPodAutoscaler"
K8S_KIND_NETWORK_POLICY = "NetworkPolicy"
K8S_KIND_NAMESPACE = "Namespace"
SECCOMP_UNCONFINED = "Unconfined"
PROC_MOUNT_UNMASKED = "Unmasked"
PULL_POLICY_NEVER = "Never"
CAPABILITY_ALL = "ALL"
CONDITION_SCALING_ACTIVE = "ScalingActive"
CONDITION_FALSE_STATUS = "False"
REASON_SCALING_DISABLED = "ScalingDisabled"
SELECT_POLICY_DISABLED = "Disabled"
CONFIG_VOLUME_SOURCES = ("configMap", "secret", "projected")
POLICY_TYPE_INGRESS = "Ingress"
PROBE_KINDS = ("livenessProbe", "readinessProbe", "startupProbe")
PROBE_PORT_HANDLERS = ("httpGet", "tcpSocket")
PROBE_SUFFIX = "Probe"
# Params that move with every usage sample: they refresh the text but never make a card 'updated'.
VOLATILE_PARAMS = ("usage", "samples", "suggested", "velocity", "restarts")
# reason -> (title, summary): the 60 v1 rules.
RECOMMENDATION_TEXT = {
    "reliability.single_replica": ("{workload} runs a single replica",
        "{workload} runs 1 replica, so a restart, a node drain or a rollout leaves it unavailable."),
    "reliability.no_pdb": ("{workload} has no disruption budget",
        "{workload} runs {replicas} replicas but no disruption budget, so a node drain may stop all of them at "
        "once."),
    "reliability.pdb_blocks_eviction": ("{workload}'s disruption budget blocks node drains",
        "The disruption budget {pdb} allows 0 disruptions while all {replicas} pods are healthy, so node drains "
        "and upgrades will hang."),
    "reliability.no_readiness_probe": ("{workload} has no readiness probe",
        "Container {containers} of {workload} serves traffic but has no readiness probe, so requests reach pods "
        "before they are ready."),
    "reliability.no_liveness_probe": ("{workload} has no liveness probe",
        "Container {containers} of {workload} has no liveness probe, so a hung process is never restarted."),
    "reliability.liveness_same_as_readiness": ("{workload} uses the same check for liveness and readiness",
        "Container {containers} of {workload} uses one check for both probes, so a slow dependency makes the "
        "kubelet restart pods instead of only taking them out of traffic."),
    "reliability.no_startup_probe": ("{workload} is restarted while starting",
        "Container {containers} of {workload} fails its liveness probe and was restarted {restarts} {timesNoun}; a "
        "slow start is the usual cause."),
    "reliability.replicas_same_node": ("All replicas of {workload} run on one node",
        "All {replicas} replicas of {workload} run on node {node}, so losing that node stops the workload."),
    "reliability.rollout_all_at_once": ("{workload} stops all replicas on every rollout",
        "{workload} uses {strategy}, so every rollout stops all {replicas} replicas before new ones are ready."),
    "reliability.short_grace_period": ("{workload} is killed without a graceful shutdown",
        "{workload} gives its pods {grace} s to stop, so in-flight requests are cut on every rollout or drain."),
    "resources.no_requests": ("{workload} has no resource requests",
        "Container {containers} of {workload} requests no {missing}, so the scheduler cannot place it reliably "
        "and it is evicted first under pressure."),
    "resources.no_memory_limit": ("{workload} has no memory limit",
        "Container {containers} of {workload} has no memory limit, so a leak can use node memory other pods "
        "need."),
    "resources.limits_without_requests": ("{workload} sets limits without requests",
        "Container {containers} of {workload} sets a {resource} limit without a request, so Kubernetes reserves "
        "the full limit."),
    "resources.memory_near_limit": ("{workload} runs close to its memory limit",
        "Container {container} of {workload} uses {usage} of its {limit} memory limit (p95 over {samples} "
        "samples), so a small spike means an out-of-memory kill."),
    "resources.cpu_near_limit": ("{workload} is likely CPU-throttled",
        "Container {container} of {workload} uses {usage} of its {limit} CPU limit (p95 over {samples} samples), "
        "so it is likely throttled and slower than it should be."),
    "resources.overprovisioned": ("{workload} requests more than it uses",
        "Container {container} of {workload} requests {request} {resource} but uses {usage} (p95 over {samples} "
        "samples); about {suggested} would do."),
    "resources.underprovisioned": ("{workload} uses more than it requests",
        "Container {container} of {workload} requests {request} {resource} but uses {usage} (p95 over {samples} "
        "samples), so it relies on capacity that is not reserved for it."),
    "resources.oom_history": ("{workload} was killed for memory with its current limit",
        "Container {container} of {workload} was OOMKilled on {lastOom} with the same {limit} memory limit it has "
        "now, so it can happen again."),
    "scaling.hpa_min_equals_max": ("{workload}'s autoscaler cannot scale",
        "The autoscaler {hpa} of {workload} has minimum and maximum {replicas}, so it never scales."),
    "scaling.hpa_missing_requests": ("{workload}'s autoscaler cannot read utilization",
        "The autoscaler {hpa} scales on {resource} utilization, but container {containers} has no {resource} "
        "request, so utilization cannot be computed and it does not scale."),
    "scaling.hpa_at_max": ("{workload} is held at its autoscaler maximum",
        "The autoscaler {hpa} wants more than its maximum of {hpaMax} replicas for {workload}."),
    "scaling.no_hpa_sustained_load": ("{workload} runs hot without an autoscaler",
        "{workload} uses {usage} of its {request} CPU request (p95 over {samples} samples) and has no autoscaler."),
    "security.privileged": ("{workload} runs privileged containers",
        "Container {containers} of {workload} runs privileged, so a compromise of it controls the node."),
    "security.privilege_escalation_allowed": ("{workload} allows privilege escalation",
        "Container {containers} of {workload} does not forbid privilege escalation, so a setuid binary can gain "
        "more privileges than the process started with."),
    "security.runs_as_root": ("{workload} may run as root", "Container {containers} of {workload} {modeText}."),
    "security.writable_root_fs": ("{workload} has a writable root filesystem",
        "Container {containers} of {workload} can write to its root filesystem, so an attacker can modify its "
        "binaries."),
    "security.added_capabilities": ("{workload} adds Linux capabilities",
        "Container {containers} of {workload} adds {capabilities}, which widens what a compromised process can do "
        "on the node."),
    "security.host_namespaces": ("{workload} shares the node's {namespaces}",
        "{workload} uses the node's {namespaces}, so its pods see and can reach host processes or interfaces."),
    "security.host_path": ("{workload} mounts node directories",
        "{workload} mounts {volumes} from the node, so its pods can read or change node files."),
    "security.default_service_account": ("{workload} uses the default service account",
        "{workload} runs as the namespace's default service account, so it shares any permission granted to it "
        "with every other workload there."),
    "security.token_automount": ("{workload} mounts an API token it may not need",
        "Pods of {workload} have a Kubernetes API token mounted; if the application never calls the API, it only "
        "helps an attacker."),
    "security.secrets_in_env": ("{workload} passes secrets as environment variables",
        "Container {containers} of {workload} reads secrets into environment variables ({envVars}), which leak "
        "into crash dumps, child processes and debug output."),
    "security.plaintext_secret_env": ("{workload} has secret-looking values in its spec",
        "Container {containers} of {workload} sets {envVars} as plain values in the workload spec, visible to "
        "anyone who can read it."),
    "images.mutable_tag": ("{workload} uses a moving image tag",
        "Container {containers} of {workload} uses {image}, so each restart may run a different build and "
        "rollbacks cannot restore the old one."),
    "images.pull_policy_mismatch": ("{workload}'s nodes may run different builds",
        "Container {containers} of {workload} pulls {image} only when missing ({pullPolicy}), so each node keeps "
        "whichever build it pulled first."),
    "config.duplicate_env": ("{workload} defines {envVars} twice",
        "Container {container} of {workload} defines {envVars} more than once; only the last value is used, which "
        "hides the first."),
    "networking.service_selector_mismatch": ("Service {service} selects no pods",
        "Service {service} selects {selector}, which matches no pod, so every request to it fails."),
    "networking.service_port_mismatch": ("Service {service} targets a port {workload} does not expose",
        "Service {service} sends traffic to port {targetPort}, but {workload} exposes {ports}."),
    "networking.no_network_policy": ("{workload} accepts traffic from anywhere",
        "No network policy selects the pods of {workload}, so any pod in the cluster can connect to them."),
    "change_risk.high_velocity": ("{app} changes very often",
        "{app} changed {velocity} times a day on average over the last 7 days; frequent changes are the most "
        "common source of incidents."),
    "change_risk.frequent_rollbacks": ("{app} was rolled back {rollbacks} times this week",
        "{app} was rolled back {rollbacks} times in the last 7 days, so changes reach it before they are ready."),
    "protection.production_uncovered": ("Production app {app} has no protection plan",
        "{app} runs in {namespaces}, which looks like production, but no protection plan covers it."),
    "protection.production_audit_only": ("Production app {app} is only audited",
        "Every protection plan covering {app} ({plans}) is in audit mode, so violations are reported but not "
        "blocked."),
    "consistency.image_skew": ("{workload} runs different images across namespaces",
        "{workload} runs {images} in {namespaces}; if this is not a staged rollout, one namespace is behind."),
    "reliability.revision_history_zero": ("{workload} keeps no rollout history",
        "{workload} sets revisionHistoryLimit to 0, so Kubernetes keeps no previous ReplicaSet to roll back to."),
    "reliability.deployment_paused": ("{workload}'s rollouts are paused",
        "{workload} is paused, so changes to its pod template are not rolled out until it is resumed."),
    "reliability.liveness_single_failure": ("{workload} restarts on a single failed liveness check",
        "Container {containers} of {workload} has a liveness failureThreshold of 1, so one slow or missed check "
        "restarts it."),
    "reliability.probe_port_undeclared": ("{workload} probes a port name it does not declare",
        "The {probe} probe of container {containers} of {workload} targets port {port}, which the container does "
        "not declare, so the check can never succeed."),
    "reliability.pdb_blocks_at_min_scale": ("{workload}'s disruption budget blocks drains at its minimum scale",
        "The disruption budget {pdb} requires {minAvailable} available pods, but the autoscaler {hpa} may scale "
        "{workload} down to {replicas}, so node drains will hang then."),
    "scaling.hpa_inactive": ("{workload}'s autoscaler is not scaling",
        "The autoscaler {hpa} of {workload} cannot compute its metrics ({condition}), so it keeps the current "
        "replica count."),
    "security.seccomp_unset": ("{workload} runs without a seccomp profile",
        "Container {containers} of {workload} runs without a seccomp profile, so every system call is allowed."),
    "security.capabilities_not_dropped": ("{workload} keeps the default Linux capabilities",
        "Container {containers} of {workload} does not drop the default capabilities, so a compromised process "
        "keeps powers such as NET_RAW."),
    "security.host_port": ("{workload} binds node ports",
        "Container {containers} of {workload} binds host port {ports} on its node, which exposes it outside the "
        "cluster network and allows one such pod per node."),
    "security.run_as_root_group": ("{workload} runs with the root group",
        "Container {containers} of {workload} runs with group ID 0 (root), so it can read and write files owned "
        "by the root group."),
    "security.proc_mount_unmasked": ("{workload} unmasks /proc",
        "Container {containers} of {workload} mounts /proc unmasked, which exposes kernel interfaces the runtime "
        "normally hides."),
    "images.pull_policy_never": ("{workload} never pulls its image",
        "Container {containers} of {workload} uses imagePullPolicy Never, so its pods fail on any node that does "
        "not already hold {image}."),
    "images.digest_not_pinned_production": ("Production workload {workload} is not pinned by digest",
        "Container {containers} of {workload} runs {image} by tag in production, so a re-pushed tag changes what "
        "runs without any spec change."),
    "config.subpath_no_reload": ("{workload} mounts configuration with subPath",
        "Container {containers} of {workload} mounts {volumes} with subPath, so later changes to that ConfigMap or "
        "Secret never reach the running pods."),
    "scaling.hpa_scale_down_disabled": ("{workload}'s autoscaler never scales down",
        "The autoscaler {hpa} of {workload} has scale-down disabled, so replicas added under load are never "
        "removed."),
    "networking.network_policy_allows_all": ("{workload}'s network policy admits all traffic",
        "The network policy {policy} selects the pods of {workload} but has an ingress rule without sources or "
        "ports, so it admits traffic from anywhere."),
}

# -----------------------------------------------------------------------------
# Worker (one asyncio task; jobs strictly sequential)
# -----------------------------------------------------------------------------
GROUP_START_ID = "0"
STREAM_NEW_MESSAGES = ">"
REDIS_BUSYGROUP = "BUSYGROUP"
READ_COUNT = 1
READ_BLOCK_MS = 2000
CLAIM_INTERVAL_S = 60
CLAIM_MIN_IDLE_MS = 600000
CLAIM_COUNT = 10
# Consumers of past pods: removed once idle this long with nothing pending (a live one reads every READ_BLOCK_MS).
CONSUMER_MAX_IDLE_MS = 3600000
CONSUMER_FIELD_NAME = "name"
CONSUMER_FIELD_PENDING = "pending"
CONSUMER_FIELD_IDLE = "idle"
# Discovery enqueues an automatic job before exporter holds the entry that triggered it: re-read the app this many
# times, this far apart, until its history reaches the job's generation.
APP_CATCHUP_ATTEMPTS = 3
APP_CATCHUP_INTERVAL_S = 1.0
WORKER_BACKOFF_S = 15
MS_PER_S = 1000
# The SSE subscription key of an app: '<namespace>/<name>', as ?apps= carries it.
APP_REF_TEMPLATE = "{namespace}/{name}"

LOG_GROUP_CREATE_FAILED = "worker: cannot create the consumer group, retrying: {}"
LOG_WORKER_REDIS_ERROR = "worker: redis error, backing off: {}"
LOG_JOB_UNDECODABLE = "worker: acknowledging an undecodable job: {}"
LOG_RUN_FAILED = "worker: run failed ({}): {}"
LOG_FAILURE_NOT_RECORDED = "worker: could not record a failed run: {}"
LOG_CONFIG_POLL_FAILED = "config poll failed: {}"
# The run id only: the service never logs customer namespaces or names. reason = why nothing was narrated.
LOG_RUN_TIMINGS = "run {} timings: gather={:.1f}s rules={:.3f}s narrate={:.1f}s total={:.1f}s narrated={} reason={}"

REVIEW_ID_PREFIX = "review-"
REVIEW_ID_TEMPLATE = REVIEW_ID_PREFIX + "{}"
# A job that finds a sweep review holding its app retries the inflight lock at this pace, for up to REVIEW_WALL_S.
INFLIGHT_RETRY_S = 0.5
# analyzer:review field '<ns>/<name>' -> '<generation>:<epoch s>' of the app's last review.
REVIEW_RECORD_TEMPLATE = "{generation}:{epoch}"
REVIEW_RECORD_SEPARATOR = ":"
REVIEW_FAMILIES = frozenset({FAMILY_WORKLOADS, FAMILY_SERVICES, FAMILY_PDBS, FAMILY_HPAS, FAMILY_NETPOLS, FAMILY_USAGE,
                             FAMILY_APP, FAMILY_DOCUMENT, FAMILY_PLANS, FAMILY_EVENTS})
FAMILIES_SEPARATOR = ","
INDEX_SCORE_MAX = "+inf"
SWEEP_MISSING_LISTINGS = 2
RUNNING_JOB_MESSAGES = 1
# The review id and counts only: never a namespace, a name, a finding or a value.
LOG_REVIEW = "review {} gets={} findings={} secs={:.2f} incomplete={}"
LOG_REVIEW_FAILED = "review failed: {}"
LOG_SWEEP = "sweep due={} reviewed={} removed={}"
LOG_SWEEP_FAILED = "sweep tick failed: {}"
LOG_SWEEP_LIST_FAILED = "sweep: application list unavailable, tick skipped: {}"

# -----------------------------------------------------------------------------
# In-process SSE fan-out
# -----------------------------------------------------------------------------
SSE_QUEUE_MAX = 32

# -----------------------------------------------------------------------------
# Model runtime state, pulls and validation
# -----------------------------------------------------------------------------
PULL_PROGRESS_INTERVAL_S = 1
LOG_PULL_FAILED = "model pull failed: {}"
# A pull that outlives this is cancelled; the largest catalogue model is a few GB.
PULL_DEADLINE_S = 3600

LICENSE_APACHE_2 = "Apache-2.0"
LICENSE_QWEN_RESEARCH = "Qwen Research (non-commercial)"
LICENSE_RESEARCH_WARNING = "This model is licensed for research use only; commercial use is not allowed."
# model -> (licence, warning); an unlisted model has no known licence.
# The licence table validate() reads for any typed tag, not the UI catalog: the research row stays.
LICENSES = {
    DEFAULT_ANALYZER_MODEL: (LICENSE_APACHE_2, ""),
    "qwen3:1.7b": (LICENSE_APACHE_2, ""),
    "qwen3:4b": (LICENSE_APACHE_2, ""),
    "qwen2.5:7b": (LICENSE_APACHE_2, ""),
    "qwen2.5:3b": (LICENSE_QWEN_RESEARCH, LICENSE_RESEARCH_WARNING),
}

VALIDATE_REASON_INVALID_MODEL_NAME = "invalid_model_name"
VALIDATE_REASON_MODEL_LACKS_TOOLS = "model_lacks_tools"
RUNTIME_REASON_TEMPLATE = "runtime_{}"
