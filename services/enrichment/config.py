"""Environment-based configuration. Aligns with release-manager: REDIS_HOST/PORT, OLLAMA_HOST."""

import os

from dotenv import load_dotenv

from constants import (
    AUTHZ_PERMISSIONS_PATH,
    DEFAULT_API_PORT,
    DEFAULT_AUTH_SERVICE_URL,
    DEFAULT_CACHE_PREFIX,
    DEFAULT_CACHE_TTL,
    DEFAULT_EXPORTER_SERVICE_URL,
    DEFAULT_LOG_LEVEL,
    DEFAULT_OLLAMA_HOST,
    DEFAULT_OLLAMA_MODEL,
    DEFAULT_QUEUE_KEY,
    DEFAULT_REDIS_HOST,
    DEFAULT_REDIS_PORT,
    ENV_SERVICE_TOKEN,
    GLOBALCONFIG_PATH,
)

load_dotenv()

# Redis: same env shape as release-manager (REDIS_HOST, REDIS_PORT). REDIS_URL overrides if set.
REDIS_HOST: str = os.environ.get("REDIS_HOST", DEFAULT_REDIS_HOST)
REDIS_PORT: str = os.environ.get("REDIS_PORT", DEFAULT_REDIS_PORT)
REDIS_URL: str = os.environ.get("REDIS_URL") or f"redis://{REDIS_HOST}:{REDIS_PORT}"

OLLAMA_HOST: str = os.environ.get("OLLAMA_HOST", DEFAULT_OLLAMA_HOST)
OLLAMA_MODEL: str = os.environ.get("OLLAMA_MODEL", DEFAULT_OLLAMA_MODEL)
CACHE_TTL: int = int(os.environ.get("CACHE_TTL", str(DEFAULT_CACHE_TTL)))
QUEUE_KEY: str = os.environ.get("QUEUE_KEY", DEFAULT_QUEUE_KEY)
CACHE_PREFIX: str = os.environ.get("CACHE_PREFIX", DEFAULT_CACHE_PREFIX)
LOG_LEVEL: str = os.environ.get("LOG_LEVEL", DEFAULT_LOG_LEVEL)
API_PORT: int = int(os.environ.get("API_PORT", str(DEFAULT_API_PORT)))
AUTH_SERVICE_URL: str = os.environ.get("AUTH_SERVICE_URL", DEFAULT_AUTH_SERVICE_URL)
AUTHZ_PERMISSIONS_URL: str = AUTH_SERVICE_URL.rstrip("/") + AUTHZ_PERMISSIONS_PATH

EXPORTER_SERVICE_URL: str = os.environ.get("EXPORTER_SERVICE_URL", DEFAULT_EXPORTER_SERVICE_URL)
GLOBALCONFIG_URL: str = EXPORTER_SERVICE_URL.rstrip("/") + GLOBALCONFIG_PATH
SERVICE_TOKEN: str = os.environ.get(ENV_SERVICE_TOKEN, "")

# Concurrency and pool
NUM_WORKERS: int = int(os.environ.get("NUM_WORKERS", "3"))
REDIS_POOL_SIZE: int = int(os.environ.get("REDIS_POOL_SIZE", "10"))

# Observability and DLQ
METRICS_INTERVAL_S: int = int(os.environ.get("METRICS_INTERVAL_S", "60"))
DLQ_KEY: str = os.environ.get("DLQ_KEY", "enrichment:dlq")
WORKER_SHUTDOWN_TIMEOUT_S: int = int(os.environ.get("WORKER_SHUTDOWN_TIMEOUT_S", "30"))

# Provider and API key are read from the GlobalConfig CR at runtime, not from env
# (see provider_config.py). Only the model names stay here: they are not secrets
# and rarely change per install.
ANTHROPIC_MODEL: str = os.environ.get("ANTHROPIC_MODEL", "claude-haiku-4-5-20251001")
GROQ_MODEL: str = os.environ.get("GROQ_MODEL", "llama-3.3-70b-versatile")
GEMINI_MODEL: str = os.environ.get("GEMINI_MODEL", "gemini-2.0-flash")
