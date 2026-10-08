"""The analyzer's Ollama client (httpx only).

Every function takes the shared `httpx.AsyncClient(base_url=OLLAMA_HOST)`.
Ollama is an external runtime, not a Telark peer: no service token, no envelope.
"""

from __future__ import annotations

import json
import socket
from collections.abc import Callable

import httpx

from config import ANALYZER_CONTEXT_TOKENS, ANALYZER_NUM_THREAD
from constants import (
    CHAT_TEMPERATURE,
    OLLAMA_CHAT_PATH,
    OLLAMA_CONTEXT_MARKER,
    OLLAMA_DELETE_PATH,
    OLLAMA_FIELD_CAPABILITIES,
    OLLAMA_FIELD_ERROR,
    OLLAMA_FIELD_MODELS,
    OLLAMA_FIELD_NAME,
    OLLAMA_KEEP_ALIVE,
    OLLAMA_META_TIMEOUT_S,
    OLLAMA_OVERFLOW_MARKERS,
    OLLAMA_PULL_INCOMPLETE,
    OLLAMA_PULL_PATH,
    OLLAMA_SHOW_PATH,
    OLLAMA_TAGS_PATH,
    OLLAMA_UNSUPPORTED_TOOLS_MARKER,
    PULL_STATUS_SUCCESS,
)
from models import ChatMessage, ChatResponse, PullProgress


class OllamaError(Exception):
    """Base of the client's error vocabulary."""


class OllamaAbsent(OllamaError):
    """DNS has no such host: the subchart is not installed."""


class OllamaUnreachable(OllamaError):
    """Any other transport failure (not DNS, not a timeout)."""


class OllamaTimeout(OllamaError):
    pass


class ModelMissing(OllamaError):
    pass


class ModelUnsupported(OllamaError):
    pass


class OllamaBusy(OllamaError):
    pass


class ContextOverflow(OllamaError):
    pass


def _is_dns_failure(exc: BaseException) -> bool:
    seen: set[int] = set()
    cur: BaseException | None = exc
    while cur is not None and id(cur) not in seen:
        if isinstance(cur, socket.gaierror):
            return True
        seen.add(id(cur))
        cur = cur.__cause__ or cur.__context__
    return False


def _transport_error(exc: httpx.TransportError) -> OllamaError:
    if isinstance(exc, httpx.TimeoutException):
        return OllamaTimeout(type(exc).__name__)
    if isinstance(exc, httpx.ConnectError) and _is_dns_failure(exc):
        return OllamaAbsent(type(exc).__name__)
    return OllamaUnreachable(type(exc).__name__)


def _check(resp: httpx.Response) -> None:
    if resp.is_success:
        return
    body = resp.text.lower()
    status = resp.status_code
    if status == httpx.codes.SERVICE_UNAVAILABLE:
        raise OllamaBusy(status)
    if OLLAMA_UNSUPPORTED_TOOLS_MARKER in body:
        raise ModelUnsupported(status)
    if status in (httpx.codes.BAD_REQUEST, httpx.codes.INTERNAL_SERVER_ERROR) and (
        OLLAMA_CONTEXT_MARKER in body and any(m in body for m in OLLAMA_OVERFLOW_MARKERS)
    ):
        raise ContextOverflow(status)
    if status == httpx.codes.NOT_FOUND:
        raise ModelMissing(status)
    raise OllamaError(status)


async def _request(client: httpx.AsyncClient, method: str, path: str, **kwargs) -> dict:
    try:
        resp = await client.request(method, path, **kwargs)
    except httpx.TransportError as e:
        raise _transport_error(e) from e
    _check(resp)
    # A non-JSON 2xx (a proxy page in front of a runtimeUrl) is a runtime failure like any other.
    try:
        return resp.json() or {}
    except ValueError as e:
        raise OllamaError(type(e).__name__) from e


async def tags(client: httpx.AsyncClient) -> list[str]:
    body = await _request(client, "GET", OLLAMA_TAGS_PATH, timeout=OLLAMA_META_TIMEOUT_S)
    return [m[OLLAMA_FIELD_NAME] for m in body.get(OLLAMA_FIELD_MODELS) or []]


async def show(client: httpx.AsyncClient, model: str) -> list[str]:
    """The model's capabilities."""
    body = await _request(client, "POST", OLLAMA_SHOW_PATH, json={"model": model}, timeout=OLLAMA_META_TIMEOUT_S)
    return body.get(OLLAMA_FIELD_CAPABILITIES) or []


def chat_payload(
    model: str, messages: list[ChatMessage], tools: list[dict], num_predict: int, fmt: dict | None = None
) -> dict:
    payload = {
        "model": model,
        "messages": [m.model_dump() for m in messages],
        "stream": False,
        "think": False,
        "keep_alive": OLLAMA_KEEP_ALIVE,
        # Fail loudly on overflow instead of silently dropping the prompt head.
        "truncate": False,
        "shift": False,
        "options": {
            "num_ctx": ANALYZER_CONTEXT_TOKENS,
            "temperature": CHAT_TEMPERATURE,
            "num_predict": num_predict,
            "num_thread": ANALYZER_NUM_THREAD,
        },
    }
    if tools:
        payload["tools"] = tools
    if fmt is not None:
        payload["format"] = fmt
    return payload


async def chat(client: httpx.AsyncClient, payload: dict, timeout_s: float) -> ChatResponse:
    body = await _request(client, "POST", OLLAMA_CHAT_PATH, json=payload, timeout=timeout_s)
    return ChatResponse.model_validate(body)


async def delete(client: httpx.AsyncClient, model: str) -> None:
    """A model already gone (404) counts as deleted. Ollama answers a delete with an empty body."""
    try:
        resp = await client.request("DELETE", OLLAMA_DELETE_PATH, json={"model": model}, timeout=OLLAMA_META_TIMEOUT_S)
    except httpx.TransportError as e:
        raise _transport_error(e) from e
    if resp.status_code != httpx.codes.NOT_FOUND:
        _check(resp)


async def pull(client: httpx.AsyncClient, model: str, on_progress: Callable[[PullProgress], None]) -> None:
    try:
        async with client.stream("POST", OLLAMA_PULL_PATH, json={"model": model, "stream": True}, timeout=None) as resp:
            if not resp.is_success:
                await resp.aread()
                _check(resp)
            async for line in resp.aiter_lines():
                if not line.strip():
                    continue
                data = json.loads(line)
                if data.get(OLLAMA_FIELD_ERROR):
                    raise OllamaError(data[OLLAMA_FIELD_ERROR])
                progress = PullProgress.model_validate({**data, "model": model})
                on_progress(progress)
                if progress.status == PULL_STATUS_SUCCESS:
                    return
    except httpx.TransportError as e:
        raise _transport_error(e) from e
    raise OllamaError(OLLAMA_PULL_INCOMPLETE)
