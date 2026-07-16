from __future__ import annotations

"""HTTP API for enrichment-service (validation endpoints)."""

import asyncio
from dataclasses import dataclass

import httpx
from fastapi import Depends, FastAPI
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel, Field

from config import ANTHROPIC_MODEL, GEMINI_MODEL, API_PORT

from authz import require_scope

from constants import (
    API_HOST,
    PERMISSION_LEVEL_CONTRIBUTOR,
    SCOPE_SETTINGS,
    CHATGPT_MODELS_URL,
    GROQ_MODELS_URL,
    GEMINI_OPENAI_BASE_URL,
    GEMINI_MODELS_URL,
    LOG_VALIDATE_API_KEY_FAILED,
    LOG_VALIDATE_API_KEY_OK,
    MSG_INVALID_PROVIDER,
    MSG_PROVIDER_OLLAMA_NOT_ALLOWED,
    MSG_API_KEY_REQUIRED,
    MSG_VALIDATE_FAILED,
    MSG_VALIDATE_OK,
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
        allow_origins=["http://localhost:3000"],
        allow_credentials=False,
        allow_methods=["POST", "OPTIONS"],
        allow_headers=["*"],
        max_age=600,
    )

    # Validating a key against a provider is part of configuring insights, and
    # an open endpoint here would answer "is this API key live?" for anyone.
    @app.post(
        "/provider/validate-api-key",
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
            logger.info(LOG_VALIDATE_API_KEY_OK, provider)
            return ValidateAPIKeyResponse(ok=True, reason=MSG_VALIDATE_OK)

        logger.warning(LOG_VALIDATE_API_KEY_FAILED, provider, res.reason or MSG_VALIDATE_FAILED)
        return ValidateAPIKeyResponse(ok=False, reason=res.reason or MSG_VALIDATE_FAILED)

    return app


async def _validate_models_list(url: str, api_key: str) -> _ValidationResult:
    try:
        async with httpx.AsyncClient(timeout=5) as client:
            resp = await client.get(url, headers={"Authorization": f"Bearer {api_key}"})
        if resp.status_code == 200:
            return _ValidationResult(ok=True)
        if resp.status_code in (401, 403):
            return _ValidationResult(ok=False, reason="API key rejected.")
        if resp.status_code == 400 and _looks_like_invalid_api_key(resp):
            return _ValidationResult(ok=False, reason="API key rejected.")
        return _ValidationResult(
            ok=False,
            reason=_format_validation_failure(resp),
        )
    except Exception:
        return _ValidationResult(ok=False, reason="Provider unreachable.")


async def _validate_gemini(api_key: str) -> _ValidationResult:
    try:
        # Gemini API keys are validated via the native Gemini REST API (key=...),
        # not via OAuth "Bearer" auth. Using the OpenAI-compat endpoint with a
        # Bearer API key returns 400 for both good and bad keys.
        async with httpx.AsyncClient(timeout=8) as client:
            resp = await client.get(GEMINI_MODELS_URL, params={"key": api_key})
        if resp.status_code == 200:
            return _ValidationResult(ok=True)
        if resp.status_code in (401, 403):
            return _ValidationResult(ok=False, reason="API key rejected.")
        if resp.status_code == 400 and _looks_like_invalid_api_key(resp):
            return _ValidationResult(ok=False, reason="API key rejected.")
        return _ValidationResult(ok=False, reason=_format_validation_failure(resp))
    except Exception:
        return _ValidationResult(ok=False, reason="Provider unreachable.")


async def _validate_claude(api_key: str) -> _ValidationResult:
    # Use Anthropic messages API via httpx for a real minimal call.
    url = "https://api.anthropic.com/v1/messages"
    headers = {
        "x-api-key": api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json",
    }
    payload = {
        "model": ANTHROPIC_MODEL,
        "max_tokens": 1,
        "messages": [{"role": "user", "content": "ping"}],
    }
    try:
        async with httpx.AsyncClient(timeout=8) as client:
            resp = await client.post(url, json=payload, headers=headers)
        if resp.status_code == 200:
            return _ValidationResult(ok=True)
        if resp.status_code in (401, 403):
            return _ValidationResult(ok=False, reason="API key rejected.")
        if resp.status_code == 400 and _looks_like_invalid_api_key(resp):
            return _ValidationResult(ok=False, reason="API key rejected.")
        return _ValidationResult(ok=False, reason=_format_validation_failure(resp))
    except Exception:
        return _ValidationResult(ok=False, reason="Provider unreachable.")


def _format_validation_failure(resp: httpx.Response) -> str:
    provider_msg = _extract_provider_message(resp)
    if provider_msg:
        return provider_msg
    return f"Validation failed (status {resp.status_code})."


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
    if not text:
        return None
    return text[:160]


def _looks_like_invalid_api_key(resp: httpx.Response) -> bool:
    try:
        data = resp.json()
    except Exception:
        return False

    message = _collect_error_text(data)
    if message:
        msg = message.lower()
        patterns = [
            "api key not valid",
            "api_key_invalid",
            "api key invalid",
            "invalid api key",
            "incorrect api key",
            "invalid_api_key",
            "invalid authentication",
            "invalid authorization",
            "unauthorized",
        ]
        if any(p in msg for p in patterns):
            return True

    # Gemini details: {"error": {"details": [{"reason":"API_KEY_INVALID", ...}]}}
    if isinstance(data, dict):
        err = data.get("error")
        if isinstance(err, dict):
            details = err.get("details")
            if isinstance(details, list):
                for d in details:
                    if isinstance(d, dict) and d.get("reason") == "API_KEY_INVALID":
                        return True
    return False


def _collect_error_text(data: object) -> str:
    if not isinstance(data, dict):
        return ""
    # Prefer explicit strings
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

