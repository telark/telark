"""Redis connection and key helpers for the enrichment worker."""

import json
import time

import httpx
import redis

from app_logger import logger
from constants import (
    PROMPT_VERSION_KEY,
    LOG_DLQ_REPLAYED,
    LOG_OLLAMA_NOT_READY,
    LOG_OLLAMA_READY,
    LOG_REDIS_CONNECTED,
    LOG_REDIS_NOT_READY,
    OLLAMA_BACKOFF_CAP_S,
    OLLAMA_BACKOFF_INITIAL_S,
    OLLAMA_HTTP_TIMEOUT_S,
    REDIS_BACKOFF_CAP_S,
    REDIS_BACKOFF_INITIAL_S,
)
from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION


def connect_redis(url: str) -> redis.Redis:
    """Connect to Redis with exponential backoff. Never exits."""
    delay = REDIS_BACKOFF_INITIAL_S
    attempt = 0
    while True:
        attempt += 1
        try:
            r = redis.from_url(url)
            r.ping()
            logger.info(LOG_REDIS_CONNECTED, attempt)
            return r
        except redis.RedisError as e:
            logger.info(LOG_REDIS_NOT_READY, delay, attempt, type(e).__name__)
            time.sleep(delay)
            delay = min(delay * 2, REDIS_BACKOFF_CAP_S)


def wait_for_ollama(host: str) -> None:
    delay = OLLAMA_BACKOFF_INITIAL_S
    attempt = 0
    while True:
        attempt += 1
        try:
            resp = httpx.get(f"{host.rstrip('/')}/", timeout=OLLAMA_HTTP_TIMEOUT_S)
            if resp.status_code == 200:
                logger.info(LOG_OLLAMA_READY, attempt)
                return
        except Exception as e:
            logger.info(LOG_OLLAMA_NOT_READY, delay, attempt, type(e).__name__)
        time.sleep(delay)
        delay = min(delay * 2, OLLAMA_BACKOFF_CAP_S)


def cache_key(prefix: str, namespace: str, name: str) -> str:
    return f"{prefix}:{namespace}:{name}"


def inflight_key(prefix: str, namespace: str, name: str) -> str:
    return f"{prefix}:inflight:{namespace}:{name}"


def enqueued_key(prefix: str, namespace: str, name: str) -> str:
    return f"{prefix}:enqueued:{namespace}:{name}"


def clear_inflight(r: redis.Redis, key: str) -> None:
    try:
        r.delete(key)
    except redis.RedisError:
        pass


def is_prompt_stale(cached_json: str) -> bool:
    """A missing promptVersion (pre-versioning entry) or an unparseable payload
    both count as stale, so the entry is re-enriched rather than trusted."""
    try:
        return json.loads(cached_json).get(PROMPT_VERSION_KEY, "") != PROMPT_VERSION
    except Exception:
        return True


def replay_dlq(r: redis.Redis, queue_key: str, dlq_key: str) -> int:
    """Move all DLQ jobs back to the main queue for reprocessing."""
    count = 0
    while True:
        job = r.rpoplpush(dlq_key, queue_key)
        if job is None:
            break
        count += 1
    if count > 0:
        logger.info(LOG_DLQ_REPLAYED, count)
    return count
