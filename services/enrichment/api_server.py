"""HTTP API for enrichment-service (validation endpoints)."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

import httpx
from fastapi import Depends, FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from config import ANTHROPIC_MODEL, API_PORT

from authz import require_scope, require_service_token
from insights import dispatch_applications
from models import InsightsDispatchRequest, InsightsDispatchResponse

from constants import (
    API_HOST,
    CORS_ALLOWED_ORIGIN,
    CORS_MAX_AGE_S,
    PERMISSION_LEVEL_CONTRIBUTOR,
    SCOPE_SETTINGS,
    ANTHROPIC_API_VERSION,
    ANTHROPIC_MESSAGES_URL,
    CHATGPT_MODELS_URL,
    GROQ_MODELS_URL,
    GEMINI_MODELS_URL,
    MODELS_LIST_TIMEOUT_S,
    PROVIDER_TIMEOUT_S,
    GEMINI_INVALID_KEY_REASON,
    INVALID_API_KEY_PATTERNS,
    PROVIDER_MESSAGE_MAX_LEN,
    LOG_VALIDATE_API_KEY_FAILED,
    LOG_VALIDATE_API_KEY_OK,
    MSG_API_KEY_REJECTED,
    MSG_INVALID_PROVIDER,
    MSG_PROVIDER_OLLAMA_NOT_ALLOWED,
    MSG_PROVIDER_UNREACHABLE,
    MSG_API_KEY_REQUIRED,
    MSG_VALIDATE_FAILED,
    MSG_VALIDATE_OK,
    MSG_VALIDATION_FAILED_STATUS,
    SERVICE_NAME,
    VALIDATE_API_KEY_PATH,
    STATUS_READY_PATH,
    STATUS_LIVE_PATH,
    STATUS_READY,
    STATUS_ALIVE,
    INSIGHTS_APPLICATIONS_PATH,
    HTTP_OK,
    OPERATION_SUCCESS,
    MSG_INSIGHTS_DISPATCHED,
)
from app_logger import logger


class ValidateAPIKeyRequest(BaseModel):
    provider: str = Field(..., description="gemini|groq|claude|chatgpt")
    api_key: str = Field(..., description="Provider API key")


class ValidateAPIKeyResponse(BaseModel):
    ok: bool
    reason: str | None = None


@dataclass(frozen=True)
class _ValidationResult:
    ok: bool
    reason: str | None = None


def create_app() -> FastAPI:
    app = FastAPI()
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[CORS_ALLOWED_ORIGIN],
        allow_credentials=False,
        allow_methods=["POST", "OPTIONS"],
        allow_headers=["*"],
        max_age=CORS_MAX_AGE_S,
    )

    # Validating a key against a provider is part of configuring insights, and
    # an open endpoint here would answer "is this API key live?" for anyone.
    @app.post(
        VALIDATE_API_KEY_PATH,
        response_model=ValidateAPIKeyResponse,
        dependencies=[Depends(require_scope(SCOPE_SETTINGS, PERMISSION_LEVEL_CONTRIBUTOR))],
    )
    async def validate_api_key(req: ValidateAPIKeyRequest) -> ValidateAPIKeyResponse:
        provider = (req.provider or "").strip().lower()
        api_key = (req.api_key or "").strip()

        if not api_key:
            return ValidateAPIKeyResponse(ok=False, reason=MSG_API_KEY_REQUIRED)
        if provider == "ollama":
            return ValidateAPIKeyResponse(ok=False, reason=MSG_PROVIDER_OLLAMA_NOT_ALLOWED)

        if provider == "gemini":
            res = await _validate_gemini(api_key)
        elif provider == "chatgpt":
            res = await _validate_models_list(CHATGPT_MODELS_URL, api_key)
        elif provider == "groq":
            res = await _validate_models_list(GROQ_MODELS_URL, api_key)
        elif provider == "claude":
            res = await _validate_claude(api_key)
        else:
            return ValidateAPIKeyResponse(ok=False, reason=MSG_INVALID_PROVIDER)

        if res.ok:
            logger.debug(LOG_VALIDATE_API_KEY_OK, provider)
            return ValidateAPIKeyResponse(ok=True, reason=MSG_VALIDATE_OK)

        logger.warning(LOG_VALIDATE_API_KEY_FAILED, provider, res.reason or MSG_VALIDATE_FAILED)
        return ValidateAPIKeyResponse(ok=False, reason=res.reason or MSG_VALIDATE_FAILED)

    # Probes carry no auth: kubelet reaches them with no session, and they must
    # answer while the service is still starting up.
    @app.get(STATUS_LIVE_PATH)
    async def liveness() -> dict[str, str]:
        return {"status": STATUS_ALIVE, "service": SERVICE_NAME}

    @app.get(STATUS_READY_PATH)
    async def readiness() -> dict[str, str]:
        return {"status": STATUS_READY, "service": SERVICE_NAME}

    # Discovery calls this, no user does — it carries the service token, not a
    # session. Sync def so FastAPI runs the blocking Redis work off the event loop.
    # Wrapped in the Go response envelope because the caller is a Go rest client
    # that reads success from the envelope's status field, not the HTTP code.
    @app.post(
        INSIGHTS_APPLICATIONS_PATH,
        dependencies=[Depends(require_service_token)],
    )
    def dispatch_insights(req: InsightsDispatchRequest) -> dict:
        ready, pending = dispatch_applications(req.items)
        data = InsightsDispatchResponse(ready=ready, pending=pending)
        return {
            "status": HTTP_OK,
            "operation": OPERATION_SUCCESS,
            "message": MSG_INSIGHTS_DISPATCHED,
            "data": data.model_dump(),
        }

    return app


def _classify(resp: httpx.Response) -> _ValidationResult:
    if resp.status_code == 200:
        return _ValidationResult(ok=True)
    if resp.status_code in (401, 403):
        return _ValidationResult(ok=False, reason=MSG_API_KEY_REJECTED)
    if resp.status_code == 400 and _looks_like_invalid_api_key(resp):
        return _ValidationResult(ok=False, reason=MSG_API_KEY_REJECTED)
    return _ValidationResult(ok=False, reason=_format_validation_failure(resp))


async def _validate_models_list(url: str, api_key: str) -> _ValidationResult:
    try:
        async with httpx.AsyncClient(timeout=MODELS_LIST_TIMEOUT_S) as client:
            resp = await client.get(url, headers={"Authorization": f"Bearer {api_key}"})
    except Exception:
        return _ValidationResult(ok=False, reason=MSG_PROVIDER_UNREACHABLE)
    return _classify(resp)


async def _validate_gemini(api_key: str) -> _ValidationResult:
    # Gemini keys are validated against the native REST API (key=...), not OAuth
    # "Bearer" auth: the OpenAI-compat endpoint returns 400 for good and bad keys alike.
    try:
        async with httpx.AsyncClient(timeout=PROVIDER_TIMEOUT_S) as client:
            resp = await client.get(GEMINI_MODELS_URL, params={"key": api_key})
    except Exception:
        return _ValidationResult(ok=False, reason=MSG_PROVIDER_UNREACHABLE)
    return _classify(resp)


async def _validate_claude(api_key: str) -> _ValidationResult:
    headers = {
        "x-api-key": api_key,
        "anthropic-version": ANTHROPIC_API_VERSION,
        "content-type": "application/json",
    }
    payload = {
        "model": ANTHROPIC_MODEL,
        "max_tokens": 1,
        "messages": [{"role": "user", "content": "ping"}],
    }
    try:
        async with httpx.AsyncClient(timeout=PROVIDER_TIMEOUT_S) as client:
            resp = await client.post(ANTHROPIC_MESSAGES_URL, json=payload, headers=headers)
    except Exception:
        return _ValidationResult(ok=False, reason=MSG_PROVIDER_UNREACHABLE)
    return _classify(resp)


def _format_validation_failure(resp: httpx.Response) -> str:
    return _extract_provider_message(resp) or MSG_VALIDATION_FAILED_STATUS.format(resp.status_code)


def _extract_provider_message(resp: httpx.Response) -> str | None:
    # Prefer short upstream message strings over raw JSON blobs.
    try:
        data = resp.json()
    except Exception:
        data = None

    # Common style: {"error": {"message": "..."}}
    if isinstance(data, dict):
        err = data.get("error")
        if isinstance(err, dict):
            msg = err.get("message")
            if isinstance(msg, str) and msg.strip():
                return msg.strip()
            # Some APIs use "error": {"error": "..."} or "error": {"detail": "..."}
            alt = err.get("error") or err.get("detail")
            if isinstance(alt, str) and alt.strip():
                return alt.strip()

        # Some providers: {"error": "Incorrect API key provided: ..."}
        msg = data.get("message") or data.get("error")
        if isinstance(msg, str) and msg.strip():
            return msg.strip()

    # Fallback: trimmed text (kept short)
    try:
        text = resp.text or ""
    except Exception:
        text = ""
    text = " ".join(text.split()).strip()
    return text[:PROVIDER_MESSAGE_MAX_LEN] or None


def _looks_like_invalid_api_key(resp: httpx.Response) -> bool:
    try:
        data = resp.json()
    except Exception:
        return False

    message = _collect_error_text(data).lower()
    if message and any(p in message for p in INVALID_API_KEY_PATTERNS):
        return True

    # Gemini details: {"error": {"details": [{"reason":"API_KEY_INVALID", ...}]}}
    if isinstance(data, dict):
        err = data.get("error")
        if isinstance(err, dict):
            details = err.get("details")
            if isinstance(details, list):
                return any(
                    isinstance(d, dict) and d.get("reason") == GEMINI_INVALID_KEY_REASON
                    for d in details
                )
    return False


def _collect_error_text(data: object) -> str:
    if not isinstance(data, dict):
        return ""
    for key in ("message", "error", "detail"):
        v = data.get(key)
        if isinstance(v, str) and v.strip():
            return v.strip()
    # OpenAI/Gemini/Anthropic-style nested error dict
    err = data.get("error")
    if isinstance(err, dict):
        for key in ("message", "error", "detail", "type", "code", "status"):
            v = err.get(key)
            if isinstance(v, str) and v.strip():
                return v.strip()
    return ""


def run_api_in_thread() -> None:
    import uvicorn

    config = uvicorn.Config(create_app(), host=API_HOST, port=API_PORT, log_level="info")
    server = uvicorn.Server(config=config)
    asyncio.run(server.serve())

