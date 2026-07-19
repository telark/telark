"""Central place for all text and constant values used across the enrichment service."""

# -----------------------------------------------------------------------------
# Config defaults (env fallbacks)
# -----------------------------------------------------------------------------
DEFAULT_REDIS_HOST = "localhost"
DEFAULT_REDIS_PORT = "6379"
DEFAULT_OLLAMA_HOST = "http://localhost:11434"
DEFAULT_OLLAMA_MODEL = "qwen2.5:7b"
DEFAULT_CACHE_TTL = 86400
DEFAULT_QUEUE_KEY = "enrichment:jobs"
DEFAULT_CACHE_PREFIX = "enrichment"
INFLIGHT_TTL_S = 120
DEFAULT_LOG_LEVEL = "INFO"

# -----------------------------------------------------------------------------
# API server (validation endpoint)
# -----------------------------------------------------------------------------
API_HOST = "0.0.0.0"
DEFAULT_API_PORT = 8080

CHATGPT_MODELS_URL = "https://api.openai.com/v1/models"
GROQ_MODELS_URL = "https://api.groq.com/openai/v1/models"
GEMINI_OPENAI_BASE_URL = "https://generativelanguage.googleapis.com/v1beta/openai/"
GEMINI_MODELS_URL = "https://generativelanguage.googleapis.com/v1beta/models"

MSG_PROVIDER_OLLAMA_NOT_ALLOWED = "ollama is not valid for this endpoint"
MSG_INVALID_PROVIDER = "invalid provider"
MSG_API_KEY_REQUIRED = "api_key is required"
MSG_VALIDATE_OK = "ok"
MSG_VALIDATE_FAILED = "validation failed"

SERVICE_NAME = "enrichment-service"
STATUS_READY_PATH = "/api/v1/status/ready"
STATUS_LIVE_PATH = "/api/v1/status/live"
STATUS_READY = "ready"
STATUS_ALIVE = "alive"

LOG_VALIDATE_API_KEY_OK = "API key validated for provider: {}"
LOG_VALIDATE_API_KEY_FAILED = "API key validation failed for provider {}: {}"

# -----------------------------------------------------------------------------
# Backoff and timeouts
# -----------------------------------------------------------------------------
REDIS_BACKOFF_INITIAL_S = 2
REDIS_BACKOFF_CAP_S = 30
OLLAMA_BACKOFF_INITIAL_S = 5
OLLAMA_BACKOFF_CAP_S = 30
OLLAMA_HTTP_TIMEOUT_S = 3
OLLAMA_REQUEUE_SLEEP_S = 10
MAIN_LOOP_ERROR_SLEEP_S = 1
BLPOP_TIMEOUT_S = 1
CONNECTION_ERROR_SLEEP_S = 5
RATE_LIMIT_RETRIES = 3

# -----------------------------------------------------------------------------
# Validation patterns (single source of truth for role/confidence)
# -----------------------------------------------------------------------------
# Role is free-form — model can use known roles or a short custom description
ROLE_PATTERN = None  # removed — role is now free-form
CONFIDENCE_PATTERN = "^(high|medium|low)$"
CATEGORY_PATTERN = "^(infrastructure|application|data|messaging|security)$"
SEVERITY_PATTERN = "^(high|medium|low)$"
PRIORITY_PATTERN = "^(high|medium|low)$"
EFFICIENCY_STATUS_PATTERN = "^(over|under|balanced|unknown)$"
CRITICALITY_LEVEL_PATTERN = "^(critical|high|medium|low)$"

# -----------------------------------------------------------------------------
# Log / message strings — main
# -----------------------------------------------------------------------------
LOG_SHUTTING_DOWN = "Shutting down enrichment worker..."
LOG_OLLAMA_HOST_MODEL = "Ollama host: {}, model: {}"
LOG_PROVIDER = "Provider: {}"
LOG_WORKER_READY = "Enrichment worker ready"
LOG_STARTING_LOOP = "Starting blocking loop on queue key: {}"
LOG_REDIS_BLPOP_ERROR = "Redis error during BLPOP, reconnecting: {}"
LOG_BAD_JOB = "Bad job, skipping: {}"
LOG_OLLAMA_REQUEUE = "Ollama unavailable, re-queuing job for {}/{}"
LOG_FAILED_REQUEUE = "Failed to re-queue job: {}"
LOG_ENRICHMENT_FAILED = "Enrichment failed, skipping job: {}"
LOG_ENRICHED = "Enriched {}/{} → {} ({}) in {}ms"
LOG_REDIS_SETEX_ERROR = "Redis error during SETEX, re-queuing job and reconnecting: {}"
LOG_CACHE_WRITE_FAILED = "Cache write failed: {}"
LOG_UNEXPECTED_LOOP_ERROR = "Unexpected error in main loop, continuing: {}"
LOG_REDIS_CLOSED = "Redis connection closed"
LOG_JOB_INFLIGHT = "Job already in-flight, skipping: {}/{}"
LOG_CACHE_EXISTS_SKIP = "Cache already exists for {}/{}, skipping duplicate job"
LOG_PROMPT_VERSION_STALE = "Worker {} prompt version changed, re-enriching: {}/{}"
LOG_PROMPT_VERSION = "Prompt version: {}"

# Prompt versioning (cache invalidation when prompt template changes)
PROMPT_VERSION_KEY = "promptVersion"

# -----------------------------------------------------------------------------
# Log / message strings — workers, DLQ, metrics, shutdown
# -----------------------------------------------------------------------------
LOG_WORKER_STARTED = "Worker {} started"
LOG_WORKER_STOPPED = "Worker {} stopped"
LOG_JOB_TO_DLQ = "Job moved to DLQ after failure: {}/{}"
LOG_DLQ_REPLAYED = "Replayed {} jobs from DLQ"
LOG_METRICS = "Worker {} | processed={} failed={} skipped={} avg={}ms"
LOG_SHUTDOWN_WAITING = "Shutting down: waiting up to {}s for {} workers"
LOG_SHUTDOWN_CLEANED_INFLIGHT = "Shutdown: cleaned inflight key for worker {}"

# -----------------------------------------------------------------------------
# Log / message strings — helpers
# -----------------------------------------------------------------------------
LOG_REDIS_CONNECTED = "Redis connected on attempt {}"
LOG_REDIS_NOT_READY = "Redis not ready, retrying in {}s... (attempt {}): {}"
LOG_OLLAMA_READY = "Ollama ready on attempt {}"
LOG_OLLAMA_NOT_READY = "Ollama not ready, retrying in {}s... (attempt {}): {}"
# -----------------------------------------------------------------------------
# Authorization
# -----------------------------------------------------------------------------
DEFAULT_AUTH_SERVICE_URL = "http://telark-auth-service:8080"
AUTHZ_PERMISSIONS_PATH = "/api/v1/auth/permissions"
AUTHZ_TIMEOUT_SECONDS = 5.0

HEADER_SESSION_TOKEN = "X-Session-Token"
HEADER_SERVICE_TOKEN = "X-Service-Token"
ENV_SERVICE_TOKEN = "TELARK_SERVICE_TOKEN"

# AI provider + key live in the GlobalConfig CR. Read through exporter (the sole
# CRD reader) with the service token, cached briefly so it is not fetched per job.
DEFAULT_EXPORTER_SERVICE_URL = "http://telark-exporter-service:8080"
GLOBALCONFIG_PATH = "/api/v1/resources/globalconfig/get"
GLOBALCONFIG_TIMEOUT_SECONDS = 5.0
GLOBALCONFIG_TTL_SECONDS = 45.0
LOG_GLOBALCONFIG_FETCH_FAILED = "failed to read AI config from GlobalConfig: {error}"

SCOPE_ALL = "ALL"
SCOPE_SETTINGS = "settings"

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

MSG_AI_DISABLED = "AI enrichment is not configured"

# Scope-based insights dispatch. Discovery posts a whole scope's signals in one
# call; the route returns what is cached and enqueues the rest for the workers.
INSIGHTS_APPLICATIONS_PATH = "/api/v1/insights/applications"
# The Go rest clients read success from the response envelope, not the HTTP code,
# so this internal route answers in their shape.
HTTP_OK = 200
OPERATION_SUCCESS = "success"
MSG_INSIGHTS_DISPATCHED = "insights dispatch accepted"
# Marks an app as already queued, so repeated discovery ticks do not pile the
# same job up while the workers are still draining. Self-expiring: once a worker
# produces the result the cache is fresh and future ticks see it as ready anyway;
# the TTL only bounds how long a lost job blocks a retry.
ENQUEUED_TTL_S = 600
LOG_INSIGHTS_DISPATCH = "insights dispatch: {ready} ready, {pending} queued (scope=applications)"

# Connectivity heartbeat: the Go rest clients refuse to call a service whose
# readiness key is absent, so this service must publish its own, same key shape,
# value and TTL the Go side uses.
CONNECTIVITY_KEY = "connectivity:service:enrichment"
CONNECTIVITY_VALUE_READY = "1"
CONNECTIVITY_TTL_S = 3
CONNECTIVITY_INTERVAL_S = 1

MSG_AUTHZ_SERVICE_ONLY = "this endpoint is for internal service calls only"
MSG_AUTHZ_MISSING_SESSION = "a session token is required"
MSG_AUTHZ_FORBIDDEN = "you do not have permission to perform this action"
MSG_AUTHZ_UNAVAILABLE = "unable to verify permissions"

LOG_AUTHZ_RESOLVE_FAILED = "authz: failed to resolve permissions: {error}"
