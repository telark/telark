"""Local analyzer entry point: one uvicorn server and, on the same event loop, the
config poll, the one worker task that consumes insights:jobs and the review sweep.

A fast run writes twice: the rule cards as soon as they are known (lastRun still
running), then the narrated prose, the observed resolves and the finished run; its
setup review then writes the recommendations (lastReviewAt, never lastRun).

uvicorn handles SIGTERM; timeout_graceful_shutdown keeps open SSE streams from
holding a rollout. Redis is not awaited at startup: readiness reports it.
"""

from __future__ import annotations

import asyncio
import math
import socket
import ssl
import time
from collections.abc import AsyncIterator, Awaitable, Callable
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from pathlib import Path

import httpx
import redis.asyncio as aioredis
import uvicorn
from fastapi import FastAPI
from pydantic import ValidationError
from redis import RedisError

import analyzer
import exporter
import recommendations
import review
from api_server import create_app, valid_app
from app_logger import configure, logger
from config import (
    ANALYZER_CONFIG_POLL_SEC,
    ANALYZER_REVIEW_APPS_PER_MIN,
    ANALYZER_REVIEW_INTERVAL_SEC,
    ANALYZER_REVIEW_TICK_SEC,
    API_PORT,
    LOG_LEVEL,
    OLLAMA_HOST,
    REDIS_PASSWORD,
    REDIS_POOL_SIZE,
    REDIS_URL,
)
from constants import (
    API_ERROR_INVALID_APP,
    API_HOST,
    APP_CATCHUP_ATTEMPTS,
    APP_CATCHUP_INTERVAL_S,
    APP_KEY_SEPARATOR,
    CLAIM_COUNT,
    CLAIM_INTERVAL_S,
    CLAIM_MIN_IDLE_MS,
    CONSUMER_FIELD_IDLE,
    CONSUMER_FIELD_NAME,
    CONSUMER_FIELD_PENDING,
    CONSUMER_GROUP,
    CONSUMER_MAX_IDLE_MS,
    EVENT_ANALYSIS_FAILED,
    EVENT_ANALYSIS_FINISHED,
    EVENT_ANALYSIS_STARTED,
    EVENT_INSIGHT_CREATED,
    EVENT_INSIGHT_RESOLVED,
    EVENT_INSIGHT_UPDATED,
    EVENT_REVIEW_FINISHED,
    FAMILIES_SEPARATOR,
    FAMILY_EVENTS,
    FIELD_GENERATION,
    GROUP_START_ID,
    INDEX_KEY,
    INFLIGHT_RETRY_S,
    INDEX_SCORE_MAX,
    INDEX_SCORE_MIN,
    JOB_MAX_AGE_S,
    K8S_API_BASE,
    K8S_HTTP_TIMEOUT_S,
    LOG_CONFIG_POLL_FAILED,
    LOG_FAILURE_NOT_RECORDED,
    LOG_GROUP_CREATE_FAILED,
    LOG_JOB_UNDECODABLE,
    LOG_REVIEW,
    LOG_REVIEW_FAILED,
    LOG_RUN_FAILED,
    LOG_RUN_TIMINGS,
    LOG_SWEEP,
    LOG_SWEEP_FAILED,
    LOG_SWEEP_LIST_FAILED,
    LOG_WORKER_REDIS_ERROR,
    MODE_DEEP,
    MS_PER_S,
    READ_BLOCK_MS,
    READ_COUNT,
    REDIS_BUSYGROUP,
    REVIEW_FAMILIES,
    REVIEW_ID_PREFIX,
    REVIEW_ID_TEMPLATE,
    REVIEW_KEY,
    REVIEW_RECORD_SEPARATOR,
    REVIEW_RECORD_TEMPLATE,
    REVIEW_WALL_S,
    RUNNING_JOB_MESSAGES,
    RUN_ERROR_APP_NOT_FOUND,
    RUN_ERROR_INTERNAL,
    RUN_ERROR_JOB_DROPPED,
    RUN_ERROR_JOB_EXPIRED,
    RUN_ERROR_MODEL_NOT_INSTALLED,
    RUN_ERROR_MODEL_UNSUPPORTED,
    RUN_ERROR_RUN_IN_PROGRESS,
    RUN_ERROR_RUNTIME_UNREACHABLE,
    RUN_ERROR_STORAGE_UNAVAILABLE,
    RUN_STATUS_DONE,
    RUN_STATUS_FAILED,
    RUN_STATUS_RUNNING,
    RUNTIME_STATE_ABSENT,
    RUNTIME_STATE_MODEL_MISSING,
    RUNTIME_STATE_PULLING,
    RUNTIME_STATE_READY,
    RUNTIME_STATE_UNREACHABLE,
    RUNTIME_STATE_UNSUPPORTED,
    S_PER_MINUTE,
    SA_CA_PATH,
    SHUTDOWN_GRACE_S,
    STREAM_JOBS,
    STREAM_NEW_MESSAGES,
    SWEEP_MISSING_LISTINGS,
    TIMING_GATHER,
    TIMING_RULES,
    TRIGGER_INCIDENT,
    TRIGGER_MANUAL,
    TRIGGER_RECOVERY,
    USAGE_KEY,
    WORKER_BACKOFF_S,
)
from events import Broadcaster
from exporter import AppNotFound, ExporterUnavailable
from helpers import app_ref, now_rfc3339, primary_namespace, stream_id_ms
from insights import (
    InsightStore,
    clear_pending,
    insight_id,
    merge,
    merge_recommendations,
    narrate,
    prune,
    resolve_all,
    resolve_observed,
    stamp_review,
    stamp_run,
    validate,
)
from models import AnalyzerConfig, AppInsights, Job, LastRun, MergeStats, RecStats, Run
from runtime import Runtime
from tools.k8s_tools import K8s

Sleep = Callable[[float], Awaitable[None]]

_RUNTIME_ERRORS = {
    RUNTIME_STATE_MODEL_MISSING: RUN_ERROR_MODEL_NOT_INSTALLED,
    RUNTIME_STATE_PULLING: RUN_ERROR_MODEL_NOT_INSTALLED,
    RUNTIME_STATE_UNSUPPORTED: RUN_ERROR_MODEL_UNSUPPORTED,
    RUNTIME_STATE_ABSENT: RUN_ERROR_RUNTIME_UNREACHABLE,
    RUNTIME_STATE_UNREACHABLE: RUN_ERROR_RUNTIME_UNREACHABLE,
}


def _now_ms() -> int:
    return int(time.time() * MS_PER_S)


def _publish(state, name: str, job: Job, **data) -> None:
    state.broadcaster.publish(name, app_ref(job.namespace, job.name), data)


async def _write(state, job: Job, change: Callable[[AppInsights], object]) -> AppInsights:
    """One unconditional document write: `change` applies to the freshly read document."""

    def apply(doc: AppInsights) -> bool:
        change(doc)
        return True

    doc, _changed = await state.store.update(job.namespace, job.name, apply)
    return doc


async def drop_pending(state, job: Job, run_id: str, code: str) -> None:
    """Fail this run's queued or running lastRun (never another run's), then tell the UI."""
    now = now_rfc3339()
    doc, wrote = await state.store.update(job.namespace, job.name, lambda d: clear_pending(d, run_id, code, now))
    if wrote:
        _publish(state, EVENT_ANALYSIS_FAILED, job, version=doc.version, runId=run_id, error=code)


async def _fail(state, job: Job, running: LastRun, code: str, steps: int = 0, tool_calls: int = 0) -> None:
    """Best effort: when the failure write itself fails, lastRun stays running and the UI expires it."""
    last = running.model_copy(update=dict(
        status=RUN_STATUS_FAILED, error=code, finishedAt=now_rfc3339(), steps=steps, toolCalls=tool_calls))
    try:
        doc = await _write(state, job, lambda d: stamp_run(d, last))
    except RedisError as e:
        logger.warning(LOG_FAILURE_NOT_RECORDED, type(e).__name__)
        return
    _publish(state, EVENT_ANALYSIS_FAILED, job, version=doc.version, runId=running.runId, error=code)


def _publish_cards(state, namespace: str, name: str, doc: AppInsights,
                   batches: tuple[tuple[str, list[str]], ...]) -> None:
    status = {card.id: card.status for card in doc.insights}
    for event, ids in batches:
        for iid in ids:
            state.broadcaster.publish(event, app_ref(namespace, name), dict(id=iid, version=doc.version,
                                                                             status=status[iid]))


def _finished(state, job: Job, doc: AppInsights, run_id: str, stats: MergeStats, resolved: list[str],
              truncated: bool) -> None:
    # A reopened card is an existing card that changed: it is counted as updated.
    _publish(state, EVENT_ANALYSIS_FINISHED, job, version=doc.version, runId=run_id, truncated=truncated,
             created=len(stats.created), updated=len(stats.updated + stats.reopened), resolved=len(resolved))


def _announce(state, job: Job, doc: AppInsights, run_id: str, stats: MergeStats, resolved: list[str],
              truncated: bool) -> None:
    _publish_cards(state, job.namespace, job.name, doc, ((EVENT_INSIGHT_CREATED, stats.created),
                                                         (EVENT_INSIGHT_UPDATED, stats.updated + stats.reopened),
                                                         (EVENT_INSIGHT_RESOLVED, resolved)))
    _finished(state, job, doc, run_id, stats, resolved, truncated)


async def _run_fast(state, job: Job, running: LastRun, cfg: AnalyzerConfig, run: Run) -> str:
    """Rules first, then at most one narration; a missing model or a failed narration never fails the run."""
    start = time.monotonic()
    timings: dict[str, float] = {}
    candidates = await analyzer.gather_rules(run, state.k8s, datetime.now(UTC), timings)
    now = now_rfc3339()
    valid = validate([c.to_emitted() for c in candidates], run)
    stats = MergeStats()

    def cards(doc: AppInsights) -> None:
        nonlocal stats
        stats = merge(doc, valid, now, run.truncated, job.namespace, job.name)
        prune(doc, now)
        # Still running: the version bump alone tells the UI to refetch the cards.
        stamp_run(doc, running.model_copy())

    doc = await _write(state, job, cards)
    _publish_cards(state, job.namespace, job.name, doc, ((EVENT_INSIGHT_CREATED, stats.created),
                                                         (EVENT_INSIGHT_UPDATED, stats.updated + stats.reopened)))

    narrate_start = time.monotonic()
    texts: list[tuple[str, str]] = []
    reason = ""
    # After the first write: a slow or unreachable runtime never delays the cards.
    runtime_state = await state.runtime.check(cfg.model)
    if runtime_state != RUNTIME_STATE_READY:
        state.runtime.ensure_model(cfg.model)
        reason = runtime_state
    elif candidates:
        narrated, reason = await analyzer.narrate(state.ollama_client, cfg.model, candidates)
        texts = [(n.title, n.summary) for n in narrated or []]
    narrate_s = time.monotonic() - narrate_start
    # Only this run's cards: an id validate() dropped may name an older, resolved card.
    kept = [(iid, text) for c, text in zip(candidates, texts)
            if (iid := insight_id(job.namespace, job.name, c.subject, c.params.get("namespace", job.namespace)))
            in stats.touched]
    rewritten: list[str] = []
    resolved: list[str] = []

    def finish(doc: AppInsights) -> None:
        end = now_rfc3339()
        rewritten.extend(narrate(doc, [iid for iid, _ in kept], [text for _, text in kept]))
        # A truncated run saw too little to call anything recovered.
        resolved.extend([] if run.truncated else resolve_observed(doc, run, end, stats.touched))
        stamp_run(doc, running.model_copy(update=dict(
            status=RUN_STATUS_DONE, finishedAt=end, steps=1 if rewritten else 0, toolCalls=run.tool_calls,
            truncated=run.truncated)))

    doc = await _write(state, job, finish)
    _publish_cards(state, job.namespace, job.name, doc, ((EVENT_INSIGHT_UPDATED, rewritten),
                                                         (EVENT_INSIGHT_RESOLVED, resolved)))
    _finished(state, job, doc, running.runId, stats, resolved, run.truncated)
    logger.info(LOG_RUN_TIMINGS, running.runId, timings[TIMING_GATHER], timings[TIMING_RULES], narrate_s,
                time.monotonic() - start, bool(rewritten), reason)
    return ""


async def review_app(state, namespace: str, name: str, app: dict, run: Run, review_id: str,
                     ns_cache: dict | None = None, in_run: bool = False) -> None:
    """One setup review: read-only gather, the rules, one document write (lastReviewAt + version, never lastRun),
    then the card events. Never calls the model."""
    start = time.monotonic()
    doc = await state.store.get(namespace, name)
    inputs = await review.gather(app, doc, run, state.k8s, state.exporter_client, state.redis, ns_cache, in_run)
    findings, evaluated = recommendations.evaluate(inputs, datetime.now(UTC))
    now = now_rfc3339()
    stats = RecStats()

    def apply(doc: AppInsights) -> bool:
        nonlocal stats
        stats = merge_recommendations(doc, findings, evaluated, now, namespace, name)
        prune(doc, now)
        stamp_review(doc, now)
        return True

    doc, _changed = await state.store.update(namespace, name, apply)
    generation = (app.get("history") or {}).get(FIELD_GENERATION) or 0
    await state.redis.hset(REVIEW_KEY, app_ref(namespace, name),
                           REVIEW_RECORD_TEMPLATE.format(generation=generation, epoch=int(time.time())))
    _publish_cards(state, namespace, name, doc, ((EVENT_INSIGHT_CREATED, stats.created),
                                                 (EVENT_INSIGHT_UPDATED, stats.updated + stats.reopened),
                                                 (EVENT_INSIGHT_RESOLVED, stats.resolved)))
    # The write always bumps the version (lastReviewAt), even when no card changed: announce it.
    state.broadcaster.publish(EVENT_REVIEW_FINISHED, app_ref(namespace, name), dict(version=doc.version))
    # The review id and counts only: never a namespace, a name or a finding.
    expected = REVIEW_FAMILIES if in_run else REVIEW_FAMILIES - {FAMILY_EVENTS}
    logger.info(LOG_REVIEW, review_id, inputs.gets, len(findings), time.monotonic() - start,
                FAMILIES_SEPARATOR.join(sorted(expected - inputs.complete)))


async def _review_after_run(state, job: Job, app: dict, run: Run, run_id: str) -> None:
    """The in-run review, after the run's final write; skipped while other jobs wait (the sweep catches the app up)."""
    try:
        # The running job's own message stays in the stream until it is acknowledged.
        if await state.store.queue_len() > RUNNING_JOB_MESSAGES:
            return
        await review_app(state, job.namespace, job.name, app, run, REVIEW_ID_TEMPLATE.format(run_id), in_run=True)
    except Exception as e:  # a review failure never changes the finished run
        logger.warning(LOG_REVIEW_FAILED, type(e).__name__)


async def _get_app(state, name: str, generation: int, sleep: Sleep) -> dict:
    """The Application, re-read (bounded) until its history reaches `generation`; {} when gone."""
    try:
        app = await exporter.get_application(state.exporter_client, name)
        for _ in range(APP_CATCHUP_ATTEMPTS - 1):
            if ((app.get("history") or {}).get(FIELD_GENERATION) or 0) >= generation:
                break
            await sleep(APP_CATCHUP_INTERVAL_S)
            app = await exporter.get_application(state.exporter_client, name)
        return app
    except AppNotFound:
        return {}


async def _run(state, job: Job, running: LastRun, cfg: AnalyzerConfig, sleep: Sleep) -> str:
    # Only an incident job names the change that triggered it; a manual one carries whatever the app held.
    generation = job.generation if job.trigger == TRIGGER_INCIDENT else 0
    app = await _get_app(state, job.name, generation, sleep)
    if primary_namespace(app) != job.namespace or job.namespace in cfg.excludedNamespaces:
        await state.store.delete(job.namespace, job.name)
        _publish(state, EVENT_ANALYSIS_FAILED, job, version=0, runId=running.runId, error=RUN_ERROR_APP_NOT_FOUND)
        return RUN_ERROR_APP_NOT_FOUND
    # Still behind after the re-reads: the rules then cite no change rather than an older one.
    run = Run(job.namespace, job.name, app, cfg.excludedNamespaces, generation)

    def start(doc: AppInsights) -> None:
        if doc.lastRun.runId == running.runId:
            running.queuedAt = doc.lastRun.queuedAt
        stamp_run(doc, running.model_copy())

    doc = await _write(state, job, start)
    _publish(state, EVENT_ANALYSIS_STARTED, job, version=doc.version, runId=running.runId, model=running.model)

    if job.trigger == TRIGGER_RECOVERY:
        # Code only, no model call: the workload recovered, every open insight resolves.
        now = now_rfc3339()
        resolved: list[str] = []

        def recover(doc: AppInsights) -> None:
            resolved.extend(resolve_all(doc, now))
            prune(doc, now)
            stamp_run(doc, running.model_copy(update=dict(status=RUN_STATUS_DONE, finishedAt=now)))

        doc = await _write(state, job, recover)
        _announce(state, job, doc, running.runId, MergeStats(), resolved, False)
        return ""

    if state.runtime.status.mode != MODE_DEEP:
        code = await _run_fast(state, job, running, cfg, run)
        await _review_after_run(state, job, app, run, running.runId)
        return code

    runtime_state = await state.runtime.check(cfg.model)
    if runtime_state != RUNTIME_STATE_READY:
        state.runtime.ensure_model(cfg.model)
        code = _RUNTIME_ERRORS[runtime_state]
        await _fail(state, job, running, code)
        return code

    outcome = await analyzer.run_analysis(job, run, state.ollama_client, state.k8s, cfg.model)
    if outcome.error:
        # Insights stay untouched: only lastRun records the failure.
        await _fail(state, job, running, outcome.error, outcome.steps, outcome.tool_calls)
        return outcome.error

    now = now_rfc3339()
    valid = validate(outcome.emitted, run)
    stats = MergeStats()
    resolved = []

    def apply(doc: AppInsights) -> None:
        nonlocal stats, resolved
        stats = merge(doc, valid, now, outcome.truncated, job.namespace, job.name)
        # A truncated run saw too little to call anything recovered.
        resolved = [] if outcome.truncated else resolve_observed(doc, run, now, stats.touched)
        prune(doc, now)
        stamp_run(doc, running.model_copy(update=dict(
            status=RUN_STATUS_DONE, finishedAt=now, steps=outcome.steps, toolCalls=outcome.tool_calls,
            truncated=outcome.truncated)))

    doc = await _write(state, job, apply)
    _announce(state, job, doc, running.runId, stats, resolved, outcome.truncated)
    await _review_after_run(state, job, app, run, running.runId)
    return ""


async def execute(state, job: Job, run_id: str, cfg: AnalyzerConfig, sleep: Sleep) -> str:
    """One run; returns its lastRun.error code ('' when done). A run failure never escapes."""
    taken, holder = await state.store.acquire_inflight(job.namespace, job.name, run_id)
    waited = 0.0
    # A sweep review holds the app for at most REVIEW_WALL_S: the job waits it out instead of failing.
    while not taken and holder.startswith(REVIEW_ID_PREFIX) and waited < REVIEW_WALL_S:
        await sleep(INFLIGHT_RETRY_S)
        waited += INFLIGHT_RETRY_S
        taken, holder = await state.store.acquire_inflight(job.namespace, job.name, run_id)
    if not taken:
        await drop_pending(state, job, run_id, RUN_ERROR_RUN_IN_PROGRESS)
        return RUN_ERROR_RUN_IN_PROGRESS
    running = LastRun(status=RUN_STATUS_RUNNING, trigger=job.trigger, runId=run_id, model=cfg.model,
                      startedAt=now_rfc3339())
    try:
        return await _run(state, job, running, cfg, sleep)
    except (RedisError, ExporterUnavailable) as e:
        logger.warning(LOG_RUN_FAILED, RUN_ERROR_STORAGE_UNAVAILABLE, type(e).__name__)
        await _fail(state, job, running, RUN_ERROR_STORAGE_UNAVAILABLE)
        return RUN_ERROR_STORAGE_UNAVAILABLE
    # One bad job must not end the worker task. CancelledError is not an Exception: on SIGTERM the
    # message stays pending and XAUTOCLAIM re-delivers it after the restart.
    except Exception as e:
        logger.warning(LOG_RUN_FAILED, RUN_ERROR_INTERNAL, type(e).__name__)
        await _fail(state, job, running, RUN_ERROR_INTERNAL)
        return RUN_ERROR_INTERNAL
    finally:
        await state.store.release_inflight(job.namespace, job.name, run_id)


async def handle(state, msg_id: str, fields: dict | None, cfg: AnalyzerConfig, clock_ms: Callable[[], int],
                 sleep: Sleep) -> str:
    """One stream message, always acknowledged; returns its lastRun.error code ('' when done)."""
    try:
        job = Job.model_validate(fields)
    except ValidationError as e:
        # The analyze handler never writes such a message, so no document holds its id.
        logger.warning(LOG_JOB_UNDECODABLE, type(e).__name__)
        await state.store.ack_job(msg_id)
        return ""
    # Redis is untrusted: a planted job must not steer the exporter URL or the document key.
    if not valid_app(job.namespace, job.name):
        logger.warning(LOG_JOB_UNDECODABLE, API_ERROR_INVALID_APP)
        await state.store.ack_job(msg_id)
        return ""
    if clock_ms() - stream_id_ms(msg_id) > JOB_MAX_AGE_S * MS_PER_S:
        code = RUN_ERROR_JOB_EXPIRED
    elif job.trigger != TRIGGER_MANUAL and not cfg.autoAnalyze:
        code = RUN_ERROR_JOB_DROPPED
    elif job.trigger == TRIGGER_INCIDENT and not await state.store.cooldown_auto(job.namespace, job.name):
        code = RUN_ERROR_JOB_DROPPED
    else:
        code = ""
    if code:
        await drop_pending(state, job, msg_id, code)
    else:
        # Failures are recorded in lastRun and never retried by redelivery.
        code = await execute(state, job, msg_id, cfg, sleep)
    await state.store.ack_job(msg_id)
    return code


async def _handle_batch(state, entries: list, cfg: AnalyzerConfig, clock_ms: Callable[[], int], sleep: Sleep) -> None:
    for msg_id, fields in entries:
        # An unreachable runtime fails every job at once: back off instead of draining the queue into failures.
        if await handle(state, msg_id, fields, cfg, clock_ms, sleep) == RUN_ERROR_RUNTIME_UNREACHABLE:
            await sleep(WORKER_BACKOFF_S)


async def _ensure_group(r, sleep: Sleep) -> None:
    while True:
        try:
            await r.xgroup_create(STREAM_JOBS, CONSUMER_GROUP, id=GROUP_START_ID, mkstream=True)
            return
        except RedisError as e:
            if str(e).startswith(REDIS_BUSYGROUP):
                return
            logger.warning(LOG_GROUP_CREATE_FAILED, type(e).__name__)
            await sleep(WORKER_BACKOFF_S)


async def _drop_dead_consumers(r, live: str) -> None:
    """Forget the consumers past pods left in the group: nothing pending, idle past CONSUMER_MAX_IDLE_MS."""
    for c in await r.xinfo_consumers(STREAM_JOBS, CONSUMER_GROUP):
        name = c[CONSUMER_FIELD_NAME]
        if name != live and not c[CONSUMER_FIELD_PENDING] and c[CONSUMER_FIELD_IDLE] > CONSUMER_MAX_IDLE_MS:
            await r.xgroup_delconsumer(STREAM_JOBS, CONSUMER_GROUP, name)


async def worker_loop(
    state, sleep: Sleep = asyncio.sleep, clock_ms: Callable[[], int] = _now_ms, consumer: str | None = None
) -> None:
    """The one worker task: jobs strictly sequential; idles while disabled, and in deep mode while a model pulls."""
    consumer = consumer or socket.gethostname()
    r = state.redis
    await _ensure_group(r, sleep)
    next_claim_ms = 0
    while True:
        cfg = exporter.current()
        # Fast runs go on during a pull: the rules need no model, only the narration is skipped.
        if not cfg.enabled or (state.runtime.pulling and state.runtime.status.mode == MODE_DEEP):
            await sleep(ANALYZER_CONFIG_POLL_SEC)
            continue
        try:
            if clock_ms() >= next_claim_ms:
                next_claim_ms = clock_ms() + CLAIM_INTERVAL_S * MS_PER_S
                # Messages a crashed or restarted worker left pending are handled like new ones.
                reply = await r.xautoclaim(STREAM_JOBS, CONSUMER_GROUP, consumer,
                                           min_idle_time=CLAIM_MIN_IDLE_MS, count=CLAIM_COUNT)
                await _handle_batch(state, reply[1], cfg, clock_ms, sleep)
                await _drop_dead_consumers(r, consumer)
            for _stream, entries in await r.xreadgroup(CONSUMER_GROUP, consumer, {STREAM_JOBS: STREAM_NEW_MESSAGES},
                                                       count=READ_COUNT, block=READ_BLOCK_MS):
                await _handle_batch(state, entries, cfg, clock_ms, sleep)
        except RedisError as e:
            logger.warning(LOG_WORKER_REDIS_ERROR, type(e).__name__)
            await sleep(WORKER_BACKOFF_S)


def _due(listed: dict[str, dict], records: dict[str, str], now_s: float) -> list[str]:
    """Apps to review, in order: generation changed since their last review, then the oldest review first."""
    changed, stale = [], []
    for key, app in listed.items():
        generation = str((app.get("history") or {}).get(FIELD_GENERATION) or 0)
        seen, _, epoch = (records.get(key) or "").partition(REVIEW_RECORD_SEPARATOR)
        last = float(epoch) if epoch.isdigit() else 0.0
        if seen and seen != generation:
            changed.append((last, key))
        elif now_s - last >= ANALYZER_REVIEW_INTERVAL_SEC:
            stale.append((last, key))
    return [key for _, key in sorted(changed)] + [key for _, key in sorted(stale)]


async def _cleanup(state, listed: dict[str, dict], known: set[str], missing: dict[str, int]) -> int:
    """Forget apps absent from two consecutive successful listings: document, index entry, usage and review."""
    removed = 0
    for key in known - listed.keys():
        missing[key] = missing.get(key, 0) + 1
        if missing[key] < SWEEP_MISSING_LISTINGS:
            continue
        namespace, _, name = key.partition(APP_KEY_SEPARATOR)
        await state.store.delete(namespace, name)
        await state.redis.hdel(USAGE_KEY, key)
        await state.redis.hdel(REVIEW_KEY, key)
        missing.pop(key)
        removed += 1
    for key in listed.keys() & missing.keys():
        missing.pop(key)
    return removed


async def _review_one(state, key: str, cfg: AnalyzerConfig, ns_cache: dict) -> bool:
    """One sweep review under the app's inflight lock; False when another run holds it or the app is gone."""
    namespace, _, name = key.partition(APP_KEY_SEPARATOR)
    review_id = REVIEW_ID_TEMPLATE.format(_now_ms())
    taken, _holder = await state.store.acquire_inflight(namespace, name, review_id)
    if not taken:
        return False
    try:
        app = await exporter.get_application(state.exporter_client, name)
        if primary_namespace(app) != namespace:
            return False
        await review_app(state, namespace, name, app, Run(namespace, name, app, cfg.excludedNamespaces), review_id,
                         ns_cache)
        return True
    except AppNotFound:
        return False
    finally:
        await state.store.release_inflight(namespace, name, review_id)


async def review_tick(state, missing: dict[str, int], clock: Callable[[], float] = time.time) -> None:
    """One sweep tick: list apps, forget deleted ones, review the due ones within the per-tick budget."""
    cfg = exporter.current()
    if not cfg.enabled or ANALYZER_REVIEW_INTERVAL_SEC <= 0 or await state.store.queue_len() > 0:
        return
    try:
        apps = await exporter.list_applications(state.exporter_client)
    except ExporterUnavailable as e:
        logger.warning(LOG_SWEEP_LIST_FAILED, type(e).__name__)
        return
    # An empty answer is never trusted: it would read as 'every app was deleted'.
    if not apps:
        return
    listed = {app_ref(ns, a.get("name")): a for a in apps
              if (ns := primary_namespace(a)) and a.get("name") and ns not in cfg.excludedNamespaces}
    records = await state.redis.hgetall(REVIEW_KEY)
    indexed = await state.redis.zrangebyscore(INDEX_KEY, INDEX_SCORE_MIN, INDEX_SCORE_MAX)
    removed = await _cleanup(state, listed, set(records) | set(indexed), missing)
    due = _due(listed, records, clock())
    budget = math.ceil(ANALYZER_REVIEW_APPS_PER_MIN * ANALYZER_REVIEW_TICK_SEC / S_PER_MINUTE)
    reviewed, ns_cache = 0, {}
    for key in due[:budget]:
        # Incidents and manual runs go first: the sweep yields to any queued job.
        if await state.store.queue_len() > 0:
            break
        try:
            reviewed += await _review_one(state, key, cfg, ns_cache)
        except (RedisError, ExporterUnavailable) as e:
            logger.warning(LOG_REVIEW_FAILED, type(e).__name__)
    await state.store.gc_index(_now_ms())
    logger.info(LOG_SWEEP, len(due), reviewed, removed)


async def review_loop(state, sleep: Sleep = asyncio.sleep) -> None:
    """Every ANALYZER_REVIEW_TICK_SEC: one sweep tick. Runs only while the analyzer is enabled and the interval > 0."""
    missing: dict[str, int] = {}
    while True:
        try:
            await review_tick(state, missing)
        except Exception as e:  # the sweep outlives any single failure
            logger.warning(LOG_SWEEP_FAILED, type(e).__name__)
        await sleep(ANALYZER_REVIEW_TICK_SEC)


async def config_poll(state, sleep: Sleep = asyncio.sleep) -> None:
    """Every ANALYZER_CONFIG_POLL_SEC: refresh TelarkConfig, re-check the runtime for its model and, while the analyzer
    is enabled, pull that model if it is missing and autoPull is on (a fresh install then needs no Settings step)."""
    while True:
        try:
            await exporter.refresh(state.exporter_client)
            cfg = exporter.current()
            state.runtime.set_enabled(cfg.enabled)
            await state.runtime.check(cfg.model)
            if cfg.enabled:
                state.runtime.ensure_model(cfg.model)
        except Exception as e:  # the poll outlives any single failure
            logger.warning(LOG_CONFIG_POLL_FAILED, type(e).__name__)
        await sleep(ANALYZER_CONFIG_POLL_SEC)


def _k8s_http() -> httpx.AsyncClient | None:
    """The in-cluster API client; None outside a pod (the k8s tools then answer k8s_unavailable)."""
    if not Path(SA_CA_PATH).exists():
        return None
    return httpx.AsyncClient(
        base_url=K8S_API_BASE, verify=ssl.create_default_context(cafile=SA_CA_PATH), timeout=K8S_HTTP_TIMEOUT_S)


@asynccontextmanager
async def lifespan(app: FastAPI) -> AsyncIterator[None]:
    state = app.state
    state.store = InsightStore(state.redis)
    state.broadcaster = Broadcaster()
    state.ollama_client = httpx.AsyncClient(base_url=OLLAMA_HOST)
    state.exporter_client = httpx.AsyncClient()
    k8s_http = _k8s_http()
    state.k8s = K8s(k8s_http) if k8s_http else None
    state.runtime = Runtime(state.ollama_client, state.broadcaster)
    tasks = [asyncio.create_task(config_poll(state)), asyncio.create_task(worker_loop(state)),
             asyncio.create_task(review_loop(state))]
    yield
    if state.runtime.pull_task:
        tasks.append(state.runtime.pull_task)
    for task in tasks:
        task.cancel()
    await asyncio.gather(*tasks, return_exceptions=True)
    for client in (state.ollama_client, state.exporter_client, k8s_http):
        if client:
            await client.aclose()
    await state.redis.aclose()


async def serve() -> None:
    r = aioredis.Redis.from_url(
        REDIS_URL, password=REDIS_PASSWORD, max_connections=REDIS_POOL_SIZE, decode_responses=True
    )
    app = create_app(r, lifespan)
    config = uvicorn.Config(
        app,
        host=API_HOST,
        port=API_PORT,
        log_level=LOG_LEVEL.lower(),
        timeout_graceful_shutdown=SHUTDOWN_GRACE_S,
    )
    await uvicorn.Server(config).serve()


def run() -> None:
    configure(level=LOG_LEVEL)
    asyncio.run(serve())


if __name__ == "__main__":
    run()
