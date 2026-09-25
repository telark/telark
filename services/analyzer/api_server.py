"""HTTP API for the local analyzer: status probes, manual analyze, triage, runtime status/validate/pull and the SSE
stream."""

from __future__ import annotations

import asyncio
import json
import re
from collections.abc import AsyncIterator, Callable
from contextlib import AbstractAsyncContextManager

from fastapi import Depends, FastAPI, Request, status
from fastapi.exceptions import RequestValidationError
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse, StreamingResponse
from pydantic import BaseModel
from redis import RedisError
from starlette.exceptions import HTTPException

import exporter
from app_logger import logger
from authz import require_any
from config import ANALYZER_QUEUE_MAX, OLLAMA_AUTO_PULL
from constants import (
    ACTION_ANALYZE_INSIGHTS,
    ACTION_CONTROL_AI_INSIGHTS,
    ACTION_TRIAGE_INSIGHTS,
    ANALYZE_PATH,
    API_ERROR_ANALYZER_DISABLED,
    API_ERROR_AUTO_PULL_DISABLED,
    API_ERROR_COOLDOWN_ACTIVE,
    API_ERROR_INVALID_APP,
    API_ERROR_INVALID_REQUEST,
    API_ERROR_QUEUE_FULL,
    APP_KEY_SEPARATOR,
    APP_REF_TEMPLATE,
    APPS_SEPARATOR,
    CORS_ALLOWED_HEADERS,
    CORS_ALLOWED_METHODS,
    CORS_ALLOWED_ORIGIN,
    CORS_MAX_AGE_S,
    DNS1123_LABEL_PATTERN,
    ENVELOPE_CODE,
    ENVELOPE_DATA,
    ENVELOPE_MESSAGE,
    ENVELOPE_OPERATION,
    ENVELOPE_STATUS,
    EVENT_ANALYSIS_QUEUED,
    EVENT_INSIGHT_UPDATED,
    EVENTS_PATH,
    FIELD_GENERATION,
    FIELD_NAME,
    FIELD_NAMESPACE,
    FIELD_TRIGGER,
    LOG_ANALYZE_FAILED,
    LOG_QUEUED_NOT_STAMPED,
    LOG_TRIAGE_FAILED,
    METHOD_GET,
    METHOD_POST,
    MODE_DEEP,
    MODEL_NAME_PATTERN,
    OPERATION_ERROR,
    OPERATION_SUCCESS,
    PERMISSION_LEVEL_CONTRIBUTOR,
    PERMISSION_LEVEL_OWNER,
    PERMISSION_LEVEL_READONLY,
    PROBE_KEY_SERVICE,
    PROBE_KEY_STATUS,
    READY_PING_TIMEOUT_S,
    REVIEW_ID_PREFIX,
    RUN_ERROR_APP_NOT_FOUND,
    RUN_ERROR_MODEL_NOT_INSTALLED,
    RUN_ERROR_STORAGE_UNAVAILABLE,
    RUN_STATUS_QUEUED,
    RUN_STATUS_RUNNING,
    RUNTIME_PATH,
    RUNTIME_PULL_PATH,
    RUNTIME_REASON_TEMPLATE,
    RUNTIME_STATE_ABSENT,
    RUNTIME_STATE_READY,
    RUNTIME_STATE_UNREACHABLE,
    RUNTIME_VALIDATE_PATH,
    SCOPE_INSIGHTS,
    SCOPE_SETTINGS,
    SERVICE_NAME,
    SSE_CONNECTED,
    SSE_EVENT_TEMPLATE,
    SSE_HEADERS,
    SSE_MEDIA_TYPE,
    SSE_PING,
    SSE_PING_S,
    STATUS_ALIVE,
    STATUS_LIVE_PATH,
    STATUS_NOT_READY,
    STATUS_READY,
    STATUS_READY_PATH,
    STREAM_JOBS,
    STREAM_MAX_LEN,
    TRIAGE_ERROR_INVALID,
    TRIAGE_ERROR_NOT_FOUND,
    TRIAGE_PATH,
    TRIGGER_MANUAL,
    VALIDATE_REASON_INVALID_MODEL_NAME,
    VALIDATE_REASON_MODEL_LACKS_TOOLS,
)
from events import Broadcaster
from exporter import AppNotFound, ExporterUnavailable
from helpers import app_ref, inflight_key, now_rfc3339, primary_namespace
from insights import TriageError, apply_triage, mark_queued
from models import AnalyzeResponse, AppInsights, Subscription, TriageRequest, ValidateModelRequest

# (method, path) -> alternatives, any one of which admits the caller. Each is (scope, min level, denyable
# action): an x-ware Read/Write/Own/Denyable requirement. The status probes are public and absent on purpose.
_CONTROL_AI = (SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, ACTION_CONTROL_AI_INSIGHTS)
# The Settings AI section streams and reads the runtime too, for users who may not see insights.
_INSIGHTS_OR_SETTINGS_READ = (
    (SCOPE_INSIGHTS, PERMISSION_LEVEL_READONLY, None),
    (SCOPE_SETTINGS, PERMISSION_LEVEL_OWNER, None),
)
ROUTE_REQUIREMENTS: dict[tuple[str, str], tuple[tuple[str, str, str | None], ...]] = {
    (METHOD_POST, ANALYZE_PATH): ((SCOPE_INSIGHTS, PERMISSION_LEVEL_CONTRIBUTOR, ACTION_ANALYZE_INSIGHTS),),
    (METHOD_GET, EVENTS_PATH): _INSIGHTS_OR_SETTINGS_READ,
    (METHOD_GET, RUNTIME_PATH): _INSIGHTS_OR_SETTINGS_READ,
    (METHOD_POST, RUNTIME_VALIDATE_PATH): (_CONTROL_AI,),
    (METHOD_POST, RUNTIME_PULL_PATH): (_CONTROL_AI,),
    (METHOD_POST, TRIAGE_PATH): ((SCOPE_INSIGHTS, PERMISSION_LEVEL_CONTRIBUTOR, ACTION_TRIAGE_INSIGHTS),),
}

# TriageError code -> HTTP status.
_TRIAGE_STATUS = {
    TRIAGE_ERROR_NOT_FOUND: status.HTTP_404_NOT_FOUND,
    TRIAGE_ERROR_INVALID: status.HTTP_409_CONFLICT,
}

# runtime.validate reason -> HTTP status; '' (ready) is 200.
_VALIDATE_STATUS = {
    VALIDATE_REASON_INVALID_MODEL_NAME: status.HTTP_400_BAD_REQUEST,
    RUNTIME_REASON_TEMPLATE.format(RUNTIME_STATE_ABSENT): status.HTTP_503_SERVICE_UNAVAILABLE,
    RUNTIME_REASON_TEMPLATE.format(RUNTIME_STATE_UNREACHABLE): status.HTTP_503_SERVICE_UNAVAILABLE,
    RUN_ERROR_MODEL_NOT_INSTALLED: status.HTTP_404_NOT_FOUND,
    VALIDATE_REASON_MODEL_LACKS_TOOLS: status.HTTP_422_UNPROCESSABLE_CONTENT,
}


def envelope(status_code: int, data: BaseModel | dict | None = None, message: str = "") -> JSONResponse:
    """The Go response envelope (internal/rest/response/base.go:29-34); message and data are omitempty."""
    body: dict = {
        ENVELOPE_STATUS: status_code,
        ENVELOPE_OPERATION: OPERATION_SUCCESS if status_code < status.HTTP_400_BAD_REQUEST else OPERATION_ERROR,
    }
    if message:
        body[ENVELOPE_MESSAGE] = message
    if data is not None:
        body[ENVELOPE_DATA] = data.model_dump() if isinstance(data, BaseModel) else data
    return JSONResponse(body, status_code=status_code)


def _error(status_code: int, code: str) -> JSONResponse:
    return envelope(status_code, {ENVELOPE_CODE: code})


def parse_apps(raw: str, excluded: list[str]) -> set[str]:
    """?apps= exactly as discovery parses it, minus the apps whose namespace is excluded."""
    kept = set()
    for key in (part.strip() for part in raw.split(APPS_SEPARATOR)):
        namespace, sep, _name = key.partition(APP_KEY_SEPARATOR)
        if sep and namespace not in excluded:
            kept.add(key)
    return kept


async def event_stream(broadcaster: Broadcaster, sub: Subscription, ping_s: float = SSE_PING_S) -> AsyncIterator[str]:
    """The SSE relay of one subscription. Starlette cancels it on client disconnect; finally unsubscribes."""
    try:
        yield SSE_CONNECTED
        while True:
            try:
                name, data = await asyncio.wait_for(sub.queue.get(), ping_s)
            except TimeoutError:
                yield SSE_PING
                continue
            yield SSE_EVENT_TEMPLATE.format(name=name, data=json.dumps(data))
    finally:
        broadcaster.unsubscribe(sub)


async def _analyze(state, namespace: str, name: str) -> JSONResponse:
    cfg = exporter.current()
    if not cfg.enabled:
        return _error(status.HTTP_409_CONFLICT, API_ERROR_ANALYZER_DISABLED)
    if namespace in cfg.excludedNamespaces:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    try:
        app = await exporter.get_application(state.exporter_client, name)
    except AppNotFound:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    if primary_namespace(app) != namespace:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    runtime_state = state.runtime.status.state
    # Fast mode's rules need no model: only the deep loop is refused while the runtime is not ready.
    if state.runtime.status.mode == MODE_DEEP and runtime_state != RUNTIME_STATE_READY:
        return _error(status.HTTP_503_SERVICE_UNAVAILABLE, RUNTIME_REASON_TEMPLATE.format(runtime_state))
    holder = await state.redis.get(inflight_key(namespace, name))
    # A sweep review is not the run the user asked for: the job is queued and waits for the review to end.
    if holder and not holder.startswith(REVIEW_ID_PREFIX):
        return envelope(status.HTTP_202_ACCEPTED, AnalyzeResponse(runId=holder, status=RUN_STATUS_RUNNING))
    # Before the cooldown claim, so a request refused for a full queue does not start a cooldown.
    if await state.store.queue_len() >= ANALYZER_QUEUE_MAX:
        return _error(status.HTTP_429_TOO_MANY_REQUESTS, API_ERROR_QUEUE_FULL)
    if not await state.store.cooldown_manual(namespace, name):
        return _error(status.HTTP_429_TOO_MANY_REQUESTS, API_ERROR_COOLDOWN_ACTIVE)
    generation = str((app.get("history") or {}).get(FIELD_GENERATION) or 0)
    run_id = await state.redis.xadd(
        STREAM_JOBS,
        {FIELD_NAMESPACE: namespace, FIELD_NAME: name, FIELD_TRIGGER: TRIGGER_MANUAL, FIELD_GENERATION: generation},
        maxlen=STREAM_MAX_LEN,
        approximate=True,
    )
    now = now_rfc3339()
    try:
        doc, wrote = await state.store.update(namespace, name, lambda d: mark_queued(d, run_id, now))
    except RedisError as e:
        # The job is queued either way; the worker stamps running when it takes it.
        logger.warning(LOG_QUEUED_NOT_STAMPED, type(e).__name__)
    else:
        if wrote:
            state.broadcaster.publish(
                EVENT_ANALYSIS_QUEUED,
                APP_REF_TEMPLATE.format(namespace=namespace, name=name),
                dict(runId=run_id, trigger=TRIGGER_MANUAL, version=doc.version),
            )
    return envelope(status.HTTP_202_ACCEPTED, AnalyzeResponse(runId=run_id, status=RUN_STATUS_QUEUED))


async def _triage(state, namespace: str, name: str, iid: str, action: str, user_id: str) -> JSONResponse:
    """Acknowledge, dismiss or reopen one card; the analyzer is the document's only writer, on or off."""
    if namespace in exporter.current().excludedNamespaces:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    try:
        app = await exporter.get_application(state.exporter_client, name)
    except AppNotFound:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    if primary_namespace(app) != namespace:
        return _error(status.HTTP_404_NOT_FOUND, RUN_ERROR_APP_NOT_FOUND)
    now = now_rfc3339()
    result: list = []

    def apply(doc: AppInsights) -> bool:
        card, changed = apply_triage(doc, iid, action, user_id, now)
        result.append(card)
        return changed

    try:
        doc, changed = await state.store.update(namespace, name, apply)
    except TriageError as e:
        return _error(_TRIAGE_STATUS[e.code], e.code)
    (card,) = result
    if changed:
        state.broadcaster.publish(EVENT_INSIGHT_UPDATED, app_ref(namespace, name),
                                  dict(id=card.id, version=doc.version, status=card.status))
    return envelope(status.HTTP_200_OK, card)


def create_app(
    redis_client,
    lifespan: Callable[[FastAPI], AbstractAsyncContextManager[None]] | None = None,
) -> FastAPI:
    app = FastAPI(lifespan=lifespan)
    app.state.redis = redis_client
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[CORS_ALLOWED_ORIGIN],
        allow_credentials=False,
        allow_methods=CORS_ALLOWED_METHODS,
        allow_headers=CORS_ALLOWED_HEADERS,
        max_age=CORS_MAX_AGE_S,
    )

    @app.exception_handler(HTTPException)
    async def http_error(request: Request, exc: HTTPException) -> JSONResponse:
        return envelope(exc.status_code, message=str(exc.detail))

    @app.exception_handler(RequestValidationError)
    async def invalid_request(request: Request, exc: RequestValidationError) -> JSONResponse:
        return _error(status.HTTP_400_BAD_REQUEST, API_ERROR_INVALID_REQUEST)

    # One dependency per route: a route that also takes its value (the userID) shares the cached call.
    guards = {key: require_any(*alternatives) for key, alternatives in ROUTE_REQUIREMENTS.items()}
    app.state.guards = guards

    def guard(method: str, path: str) -> list:
        return [Depends(guards[(method, path)])]

    # Probes carry no auth: kubelet reaches them with no session.
    @app.get(STATUS_LIVE_PATH)
    async def liveness() -> dict[str, str]:
        return {PROBE_KEY_STATUS: STATUS_ALIVE, PROBE_KEY_SERVICE: SERVICE_NAME}

    # Readiness is Redis only: an absent or failing model runtime must never
    # take the service out of rotation (the runtime state is reported instead).
    @app.get(STATUS_READY_PATH)
    async def readiness() -> JSONResponse:
        try:
            async with asyncio.timeout(READY_PING_TIMEOUT_S):
                await app.state.redis.ping()
        except (RedisError, OSError, TimeoutError):
            return JSONResponse(
                {PROBE_KEY_STATUS: STATUS_NOT_READY, PROBE_KEY_SERVICE: SERVICE_NAME},
                status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            )
        return JSONResponse({PROBE_KEY_STATUS: STATUS_READY, PROBE_KEY_SERVICE: SERVICE_NAME})

    @app.post(ANALYZE_PATH, dependencies=guard(METHOD_POST, ANALYZE_PATH))
    async def analyze(namespace: str, name: str) -> JSONResponse:
        if not (re.fullmatch(DNS1123_LABEL_PATTERN, namespace) and re.fullmatch(DNS1123_LABEL_PATTERN, name)):
            return _error(status.HTTP_400_BAD_REQUEST, API_ERROR_INVALID_APP)
        try:
            return await _analyze(app.state, namespace, name)
        except (RedisError, ExporterUnavailable) as e:
            logger.warning(LOG_ANALYZE_FAILED, type(e).__name__)
            return _error(status.HTTP_503_SERVICE_UNAVAILABLE, RUN_ERROR_STORAGE_UNAVAILABLE)

    @app.post(TRIAGE_PATH, dependencies=guard(METHOD_POST, TRIAGE_PATH))
    async def triage(namespace: str, name: str, id: str, body: TriageRequest,
                     user_id: str | None = Depends(guards[(METHOD_POST, TRIAGE_PATH)])) -> JSONResponse:
        if not (re.fullmatch(DNS1123_LABEL_PATTERN, namespace) and re.fullmatch(DNS1123_LABEL_PATTERN, name)):
            return _error(status.HTTP_400_BAD_REQUEST, API_ERROR_INVALID_APP)
        try:
            return await _triage(app.state, namespace, name, id, body.action, user_id or "")
        except (RedisError, ExporterUnavailable) as e:
            logger.warning(LOG_TRIAGE_FAILED, type(e).__name__)
            return _error(status.HTTP_503_SERVICE_UNAVAILABLE, RUN_ERROR_STORAGE_UNAVAILABLE)

    @app.get(EVENTS_PATH, dependencies=guard(METHOD_GET, EVENTS_PATH))
    async def events(apps: str = "") -> StreamingResponse:
        broadcaster = app.state.broadcaster
        sub = broadcaster.subscribe(parse_apps(apps, exporter.current().excludedNamespaces))
        return StreamingResponse(event_stream(broadcaster, sub), media_type=SSE_MEDIA_TYPE, headers=SSE_HEADERS)

    @app.get(RUNTIME_PATH, dependencies=guard(METHOD_GET, RUNTIME_PATH))
    async def runtime_status() -> JSONResponse:
        return envelope(status.HTTP_200_OK, app.state.runtime.status)

    @app.post(RUNTIME_VALIDATE_PATH, dependencies=guard(METHOD_POST, RUNTIME_VALIDATE_PATH))
    async def validate_model(body: ValidateModelRequest) -> JSONResponse:
        resp = await app.state.runtime.validate(body.model)
        if resp.ok:
            return envelope(status.HTTP_200_OK, resp)
        # The licence and capabilities still go back with the error code.
        return envelope(_VALIDATE_STATUS[resp.reason], {**resp.model_dump(), ENVELOPE_CODE: resp.reason})

    @app.post(RUNTIME_PULL_PATH, dependencies=guard(METHOD_POST, RUNTIME_PULL_PATH))
    async def pull_model(body: ValidateModelRequest) -> JSONResponse:
        if not re.fullmatch(MODEL_NAME_PATTERN, body.model):
            return _error(status.HTTP_400_BAD_REQUEST, VALIDATE_REASON_INVALID_MODEL_NAME)
        if not OLLAMA_AUTO_PULL:
            return _error(status.HTTP_409_CONFLICT, API_ERROR_AUTO_PULL_DISABLED)
        # A no-op while a pull runs: the answer is then that pull's status.
        app.state.runtime.start_pull(body.model)
        return envelope(status.HTTP_202_ACCEPTED, app.state.runtime.status)

    return app
