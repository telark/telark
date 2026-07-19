from __future__ import annotations

"""Enrichment worker: concurrent workers, BLPOP from pool, enrich, SETEX cache, DLQ."""

import json
import signal
import threading
import time
from concurrent.futures import ThreadPoolExecutor, wait
from dataclasses import dataclass
from datetime import UTC, datetime

import redis

from app_logger import configure, logger
from config import (
    CACHE_PREFIX,
    CACHE_TTL,
    DLQ_KEY,
    LOG_LEVEL,
    METRICS_INTERVAL_S,
    NUM_WORKERS,
    OLLAMA_HOST,
    OLLAMA_MODEL,
    QUEUE_KEY,
    REDIS_POOL_SIZE,
    REDIS_URL,
    WORKER_SHUTDOWN_TIMEOUT_S,
)
from provider_config import get_ai_config

OLLAMA = "ollama"
from constants import (
    BLPOP_TIMEOUT_S,
    CONNECTION_ERROR_SLEEP_S,
    CONNECTIVITY_INTERVAL_S,
    CONNECTIVITY_KEY,
    CONNECTIVITY_TTL_S,
    CONNECTIVITY_VALUE_READY,
    INFLIGHT_TTL_S,
    LOG_BAD_JOB,
    LOG_CACHE_EXISTS_SKIP,
    LOG_CACHE_WRITE_FAILED,
    LOG_ENRICHED,
    LOG_ENRICHMENT_FAILED,
    LOG_FAILED_REQUEUE,
    LOG_JOB_INFLIGHT,
    LOG_METRICS,
    LOG_OLLAMA_HOST_MODEL,
    LOG_OLLAMA_REQUEUE,
    LOG_PROVIDER,
    LOG_PROMPT_VERSION,
    LOG_PROMPT_VERSION_STALE,
    LOG_REDIS_BLPOP_ERROR,
    LOG_REDIS_CLOSED,
    LOG_REDIS_SETEX_ERROR,
    LOG_SHUTDOWN_CLEANED_INFLIGHT,
    LOG_SHUTDOWN_WAITING,
    LOG_SHUTTING_DOWN,
    LOG_STARTING_LOOP,
    LOG_UNEXPECTED_LOOP_ERROR,
    LOG_WORKER_STARTED,
    LOG_WORKER_STOPPED,
    LOG_WORKER_READY,
    LOG_JOB_TO_DLQ,
    MAIN_LOOP_ERROR_SLEEP_S,
    OLLAMA_REQUEUE_SLEEP_S,
    RATE_LIMIT_RETRIES,
    PROMPT_VERSION_KEY,
)
from enricher import OllamaUnavailableError, enrich
from helpers import (
    cache_key,
    clear_inflight,
    connect_redis,
    inflight_key,
    is_prompt_stale,
    wait_for_ollama,
)
from models import AppSignals, EnrichmentResult
from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION
from providers import get_provider
from api_server import run_api_in_thread

_shutdown = False
_metrics: dict[int, "WorkerMetrics"] = {}
_metrics_lock = threading.Lock()
_worker_inflight: dict[int, str] = {}
_worker_inflight_lock = threading.Lock()


@dataclass
class WorkerMetrics:
    """Per-worker metrics for visibility."""

    worker_id: int
    jobs_processed: int = 0
    jobs_failed: int = 0
    jobs_skipped: int = 0
    total_duration_ms: int = 0
    avg_duration_ms: float = 0.0
    last_job_at: datetime | None = None

    def record_processed(self, elapsed_ms: int) -> None:
        self.jobs_processed += 1
        self.total_duration_ms += elapsed_ms
        self.avg_duration_ms = self.total_duration_ms / self.jobs_processed
        self.last_job_at = datetime.now(UTC)

    def record_failed(self) -> None:
        self.jobs_failed += 1

    def record_skipped(self) -> None:
        self.jobs_skipped += 1


def get_num_workers() -> int:
    """Ollama is single-threaded; cloud providers allow concurrency.

    Resolved from GlobalConfig at startup. Switching provider family later takes a
    restart to change the worker count, though not to change which key is used.
    """
    if get_ai_config().provider == OLLAMA:
        return 1
    return NUM_WORKERS


def _is_rate_limit_error(exc: BaseException) -> bool:
    msg = str(exc).lower()
    return any(k in msg for k in ("rate", "quota", "429", "too many requests"))


def _is_connection_error(exc: BaseException) -> bool:
    msg = str(exc).lower()
    return any(
        k in msg
        for k in (
            "connection",
            "timeout",
            "unreachable",
            "refused",
            "network",
        )
    )


def _push_to_dlq(r: redis.Redis, raw: str, namespace: str, name: str, worker_id: int) -> None:
    try:
        r.lpush(DLQ_KEY, raw)
        logger.error("Worker {} " + LOG_JOB_TO_DLQ, worker_id, namespace, name)
    except redis.RedisError as e:
        logger.exception("Worker {} failed to push job to DLQ: {}", worker_id, e)


def _process_one_job(
    worker_id: int,
    r: redis.Redis,
    provider,
    raw: str,
    ckey: str,
    if_key: str,
    metrics: WorkerMetrics,
) -> None:
    """Process a single job; updates metrics and handles DLQ on final failure."""
    try:
        job = json.loads(raw)
        signals = AppSignals.model_validate(job)
    except (json.JSONDecodeError, Exception) as e:
        clear_inflight(r, if_key)
        with _worker_inflight_lock:
            _worker_inflight.pop(worker_id, None)
        logger.exception("Worker {} " + LOG_BAD_JOB, worker_id, e)
        with _metrics_lock:
            metrics.record_failed()
        _push_to_dlq(r, raw, "?", "?", worker_id)
        return

    # Rate-limit retries with backoff
    last_error = None
    for attempt in range(RATE_LIMIT_RETRIES):
        try:
            start = time.perf_counter()
            enriched = enrich(signals, provider=provider)
            elapsed_ms = int((time.perf_counter() - start) * 1000)
            break
        except OllamaUnavailableError:
            clear_inflight(r, if_key)
            with _worker_inflight_lock:
                _worker_inflight.pop(worker_id, None)
            logger.warning("Worker {} " + LOG_OLLAMA_REQUEUE, worker_id, signals.namespace, signals.name)
            try:
                r.lpush(QUEUE_KEY, raw)
            except redis.RedisError as re_err:
                logger.error("Worker {} " + LOG_FAILED_REQUEUE, worker_id, re_err)
            with _metrics_lock:
                metrics.record_failed()
            return
        except Exception as e:
            last_error = e
            if _is_connection_error(e):
                clear_inflight(r, if_key)
                with _worker_inflight_lock:
                    _worker_inflight.pop(worker_id, None)
                try:
                    r.lpush(QUEUE_KEY, raw)
                except redis.RedisError:
                    pass
                logger.warning("Worker {} connection error, re-queued job: {}", worker_id, e)
                time.sleep(CONNECTION_ERROR_SLEEP_S)
                return
            if _is_rate_limit_error(e) and attempt < RATE_LIMIT_RETRIES - 1:
                backoff = 2**attempt
                logger.warning("Worker {} rate limit (attempt {}), backoff {}s: {}", worker_id, attempt + 1, backoff, e)
                time.sleep(backoff)
                continue
            clear_inflight(r, if_key)
            with _worker_inflight_lock:
                _worker_inflight.pop(worker_id, None)
            logger.exception("Worker {} " + LOG_ENRICHMENT_FAILED, worker_id, e)
            with _metrics_lock:
                metrics.record_failed()
            _push_to_dlq(r, raw, signals.namespace, signals.name, worker_id)
            return
    else:
        if last_error is not None:
            clear_inflight(r, if_key)
            with _worker_inflight_lock:
                _worker_inflight.pop(worker_id, None)
            logger.exception("Worker {} " + LOG_ENRICHMENT_FAILED, worker_id, last_error)
            with _metrics_lock:
                metrics.record_failed()
            _push_to_dlq(r, raw, signals.namespace, signals.name, worker_id)
        return

    # Write cache (inject promptVersion for cache invalidation when prompt changes)
    try:
        payload = enriched.model_dump(mode="json")
        payload[PROMPT_VERSION_KEY] = PROMPT_VERSION
        r.setex(ckey, CACHE_TTL, json.dumps(payload))
        clear_inflight(r, if_key)
        with _worker_inflight_lock:
            _worker_inflight.pop(worker_id, None)
        with _metrics_lock:
            metrics.record_processed(elapsed_ms)
        logger.info(
            "Worker {} " + LOG_ENRICHED,
            worker_id,
            signals.namespace,
            signals.name,
            enriched.role,
            enriched.confidence,
            elapsed_ms,
        )
    except redis.RedisError as e:
        clear_inflight(r, if_key)
        with _worker_inflight_lock:
            _worker_inflight.pop(worker_id, None)
        logger.warning("Worker {} " + LOG_REDIS_SETEX_ERROR, worker_id, e)
        try:
            r.lpush(QUEUE_KEY, raw)
        except redis.RedisError as re_err:
            logger.error("Worker {} " + LOG_FAILED_REQUEUE, worker_id, re_err)
            _push_to_dlq(r, raw, signals.namespace, signals.name, worker_id)
        with _metrics_lock:
            metrics.record_failed()
    except Exception as e:
        clear_inflight(r, if_key)
        with _worker_inflight_lock:
            _worker_inflight.pop(worker_id, None)
        logger.exception("Worker {} " + LOG_CACHE_WRITE_FAILED, worker_id, e)
        with _metrics_lock:
            metrics.record_failed()
        _push_to_dlq(r, raw, signals.namespace, signals.name, worker_id)


def worker_loop(worker_id: int, pool: redis.ConnectionPool) -> None:
    """Each worker runs its own BLPOP loop with a dedicated Redis connection.

    The provider is resolved per job, not once at start: it is read from
    GlobalConfig (cached), so an admin changing provider or key takes effect
    without restarting the workers.
    """
    r = redis.Redis(connection_pool=pool)
    with _metrics_lock:
        _metrics[worker_id] = WorkerMetrics(worker_id=worker_id)
    logger.info(LOG_WORKER_STARTED, worker_id)

    while not _shutdown:
        try:
            result = r.blpop(QUEUE_KEY, timeout=BLPOP_TIMEOUT_S)
        except redis.RedisError as e:
            logger.warning("Worker {} " + LOG_REDIS_BLPOP_ERROR, worker_id, e)
            time.sleep(MAIN_LOOP_ERROR_SLEEP_S)
            continue

        if result is None:
            continue

        _key, raw = result
        try:
            job = json.loads(raw)
            signals = AppSignals.model_validate(job)
        except (json.JSONDecodeError, Exception) as e:
            logger.exception("Worker {} " + LOG_BAD_JOB, worker_id, e)
            with _metrics_lock:
                _metrics[worker_id].record_failed()
            _push_to_dlq(r, raw, "?", "?", worker_id)
            continue

        # AI off, or a cloud provider with no key: nothing this worker can do with
        # the job. Drop it rather than requeue — discovery re-dispatches once AI is
        # configured, so a dropped job is retried, not lost.
        provider = get_provider()
        if provider is None:
            with _metrics_lock:
                _metrics[worker_id].record_skipped()
            continue

        ckey = cache_key(CACHE_PREFIX, signals.namespace, signals.name)
        if_key = inflight_key(CACHE_PREFIX, signals.namespace, signals.name)

        try:
            cached = r.get(ckey)
            if cached:
                if not is_prompt_stale(cached):
                    logger.debug(
                        "Worker {} " + LOG_CACHE_EXISTS_SKIP,
                        worker_id,
                        signals.namespace,
                        signals.name,
                    )
                    with _metrics_lock:
                        _metrics[worker_id].record_skipped()
                    continue
                logger.info(
                    LOG_PROMPT_VERSION_STALE,
                    worker_id,
                    signals.namespace,
                    signals.name,
                )
            if r.exists(if_key):
                logger.debug("Worker {} " + LOG_JOB_INFLIGHT, worker_id, signals.namespace, signals.name)
                with _metrics_lock:
                    _metrics[worker_id].record_skipped()
                continue
            r.setex(if_key, INFLIGHT_TTL_S, "1")
            with _worker_inflight_lock:
                _worker_inflight[worker_id] = if_key
        except redis.RedisError as e:
            logger.warning("Worker {} " + LOG_REDIS_BLPOP_ERROR, worker_id, e)
            time.sleep(MAIN_LOOP_ERROR_SLEEP_S)
            continue

        _process_one_job(worker_id, r, provider, raw, ckey, if_key, _metrics[worker_id])

    with _worker_inflight_lock:
        _worker_inflight.pop(worker_id, None)
    logger.info(LOG_WORKER_STOPPED, worker_id)


def _metrics_reporter() -> None:
    """Log per-worker metrics every METRICS_INTERVAL_S."""
    while not _shutdown:
        time.sleep(METRICS_INTERVAL_S)
        if _shutdown:
            break
        with _metrics_lock:
            for wid, m in _metrics.items():
                logger.info(
                    LOG_METRICS,
                    wid,
                    m.jobs_processed,
                    m.jobs_failed,
                    m.jobs_skipped,
                    round(m.avg_duration_ms, 1),
                )


def _connectivity_heartbeat(pool: redis.ConnectionPool) -> None:
    """Publish the readiness key the Go rest clients look for before calling us.

    Short TTL, refreshed faster than it expires, so the key disappears on its own
    if this process dies — callers then see us as unreachable rather than stale.
    """
    r = redis.Redis(connection_pool=pool)
    while not _shutdown:
        try:
            r.set(CONNECTIVITY_KEY, CONNECTIVITY_VALUE_READY, ex=CONNECTIVITY_TTL_S)
        except redis.RedisError as e:
            logger.warning("connectivity heartbeat failed: {}", e)
        time.sleep(CONNECTIVITY_INTERVAL_S)


def _handle_signal(_signum, _frame) -> None:
    global _shutdown
    _shutdown = True
    logger.info(LOG_SHUTTING_DOWN)


def run() -> None:
    """Start Redis pool, workers, and metrics reporter; wait for graceful shutdown."""
    configure(level=LOG_LEVEL)

    signal.signal(signal.SIGTERM, _handle_signal)
    signal.signal(signal.SIGINT, _handle_signal)

    startup_provider = get_ai_config().provider
    if startup_provider == OLLAMA:
        logger.info(LOG_OLLAMA_HOST_MODEL, OLLAMA_HOST, OLLAMA_MODEL)
    else:
        logger.info(LOG_PROVIDER, startup_provider or "unset")
    logger.info(LOG_PROMPT_VERSION, PROMPT_VERSION)

    connect_redis(REDIS_URL)
    pool = redis.ConnectionPool.from_url(
        REDIS_URL,
        max_connections=REDIS_POOL_SIZE,
        decode_responses=True,
    )

    if startup_provider == OLLAMA:
        wait_for_ollama(OLLAMA_HOST)

    api_thread = threading.Thread(target=run_api_in_thread, daemon=True)
    api_thread.start()

    num_workers = get_num_workers()
    logger.info(LOG_WORKER_READY)
    logger.info(LOG_STARTING_LOOP, QUEUE_KEY)

    metrics_thread = threading.Thread(target=_metrics_reporter, daemon=True)
    metrics_thread.start()

    heartbeat_thread = threading.Thread(target=_connectivity_heartbeat, args=(pool,), daemon=True)
    heartbeat_thread.start()

    with ThreadPoolExecutor(max_workers=num_workers) as executor:
        futures = [executor.submit(worker_loop, i, pool) for i in range(num_workers)]
        while not _shutdown:
            time.sleep(0.5)
        logger.info(LOG_SHUTDOWN_WAITING, WORKER_SHUTDOWN_TIMEOUT_S, num_workers)
        done, not_done = wait(futures, timeout=WORKER_SHUTDOWN_TIMEOUT_S)
        if not_done:
            for f in not_done:
                logger.warning("Worker did not finish within shutdown timeout")
            with _worker_inflight_lock:
                for wid, if_key in list(_worker_inflight.items()):
                    try:
                        conn = redis.Redis(connection_pool=pool)
                        conn.delete(if_key)
                        logger.info(LOG_SHUTDOWN_CLEANED_INFLIGHT, wid)
                    except redis.RedisError:
                        pass

    pool.disconnect()
    logger.info(LOG_REDIS_CLOSED)


if __name__ == "__main__":
    run()
