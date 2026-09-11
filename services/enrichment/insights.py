"""Scope-based insights dispatch.

Discovery posts a whole scope's signals in one call. This returns what is already
cached and enqueues the rest for the worker pool — it never calls the LLM itself,
so it stays fast no matter how many apps are in the batch. Results are read back
separately (discovery reads the same Redis cache), so this only triggers work.
"""

from __future__ import annotations

import json

import redis

from app_logger import logger
from config import CACHE_PREFIX, QUEUE_KEY, REDIS_POOL_SIZE, REDIS_URL
from constants import ENQUEUED_TTL_S, LOG_INSIGHTS_DISPATCH
from helpers import cache_key, enqueued_key, inflight_key, is_prompt_stale
from models import AppSignals

_pool: redis.ConnectionPool | None = None


def _client() -> redis.Redis:
    global _pool
    if _pool is None:
        _pool = redis.ConnectionPool.from_url(
            REDIS_URL, max_connections=REDIS_POOL_SIZE, decode_responses=True
        )
    return redis.Redis(connection_pool=_pool)


def _is_ready(r: redis.Redis, signals: AppSignals) -> bool:
    cached = r.get(cache_key(CACHE_PREFIX, signals.namespace, signals.name))
    return bool(cached) and not is_prompt_stale(cached)


def _enqueue_once(r: redis.Redis, signals: AppSignals) -> None:
    ekey = enqueued_key(CACHE_PREFIX, signals.namespace, signals.name)
    # NX marker: only the tick that wins the set enqueues the job. Also skip if a
    # worker already has it in flight.
    if r.exists(inflight_key(CACHE_PREFIX, signals.namespace, signals.name)):
        return
    if not r.set(ekey, "1", nx=True, ex=ENQUEUED_TTL_S):
        return
    r.lpush(QUEUE_KEY, json.dumps(signals.model_dump(mode="json")))


def dispatch_applications(items: list[AppSignals]) -> tuple[list[str], list[str]]:
    """Return (ready, pending) app names. Ready are cached now; pending were
    queued (or already in flight) and will fill in on later polls."""
    r = _client()
    ready: list[str] = []
    pending: list[str] = []

    for signals in items:
        if _is_ready(r, signals):
            ready.append(signals.name)
            continue
        _enqueue_once(r, signals)
        pending.append(signals.name)

    logger.debug(LOG_INSIGHTS_DISPATCH.format(ready=len(ready), pending=len(pending)))
    return ready, pending
