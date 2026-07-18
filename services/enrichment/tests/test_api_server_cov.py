"""Coverage for api_server.py — routes (auth overridden), async validators, parsers.

Run: pytest tests/test_api_server_cov.py
"""

import asyncio
import os
import sys
import types

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import api_server as A  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402


def _resp(status, payload=None, text=None):
    req = httpx.Request("GET", "https://x")
    if payload is not None:
        return httpx.Response(status, json=payload, request=req)
    return httpx.Response(status, text=text or "", request=req)


class _FakeAsyncClient:
    """Stands in for httpx.AsyncClient(...) as an async context manager."""

    def __init__(self, resp=None, exc=None):
        self._resp, self._exc = resp, exc

    async def __aenter__(self):
        return self

    async def __aexit__(self, *a):
        return False

    async def get(self, *a, **k):
        if self._exc:
            raise self._exc
        return self._resp

    async def post(self, *a, **k):
        if self._exc:
            raise self._exc
        return self._resp


def _patch_httpx(monkeypatch, resp=None, exc=None):
    monkeypatch.setattr(A.httpx, "AsyncClient", lambda *a, **k: _FakeAsyncClient(resp, exc))


# --------------------------------------------------------------------------- #
# pure parsers
# --------------------------------------------------------------------------- #
def test_collect_error_text_variants():
    assert A._collect_error_text({"message": "m"}) == "m"
    assert A._collect_error_text({"error": {"detail": "d"}}) == "d"
    assert A._collect_error_text("nope") == ""
    assert A._collect_error_text({"nothing": 1}) == ""


def test_extract_provider_message():
    assert A._extract_provider_message(_resp(400, {"error": {"message": "boom"}})) == "boom"
    assert A._extract_provider_message(_resp(400, {"error": {"detail": "d"}})) == "d"
    assert A._extract_provider_message(_resp(400, {"error": "flat"})) == "flat"
    assert A._extract_provider_message(_resp(400, text="  hi   there  ")) == "hi there"
    assert A._extract_provider_message(_resp(400, text="")) is None


def test_format_validation_failure():
    assert A._format_validation_failure(_resp(400, {"error": {"message": "x"}})) == "x"
    assert "status 418" in A._format_validation_failure(_resp(418, text=""))


def test_looks_like_invalid_api_key():
    assert A._looks_like_invalid_api_key(_resp(400, {"error": {"message": "Invalid API key"}})) is True
    assert A._looks_like_invalid_api_key(
        _resp(400, {"error": {"details": [{"reason": "API_KEY_INVALID"}]}})
    ) is True
    assert A._looks_like_invalid_api_key(_resp(400, {"error": {"message": "quota"}})) is False
    assert A._looks_like_invalid_api_key(_resp(400, text="notjson{")) is False


# --------------------------------------------------------------------------- #
# async validators
# --------------------------------------------------------------------------- #
def test_validate_models_list_paths(monkeypatch):
    _patch_httpx(monkeypatch, _resp(200, {}))
    assert asyncio.run(A._validate_models_list("u", "k")).ok is True
    _patch_httpx(monkeypatch, _resp(401, {}))
    assert asyncio.run(A._validate_models_list("u", "k")).ok is False
    _patch_httpx(monkeypatch, _resp(400, {"error": {"message": "invalid api key"}}))
    assert asyncio.run(A._validate_models_list("u", "k")).ok is False
    _patch_httpx(monkeypatch, _resp(500, {"error": {"message": "server"}}))
    assert asyncio.run(A._validate_models_list("u", "k")).ok is False
    _patch_httpx(monkeypatch, exc=RuntimeError("unreachable"))
    assert "unreachable" in asyncio.run(A._validate_models_list("u", "k")).reason.lower()


def test_validate_gemini_and_claude(monkeypatch):
    _patch_httpx(monkeypatch, _resp(200, {}))
    assert asyncio.run(A._validate_gemini("k")).ok is True
    assert asyncio.run(A._validate_claude("k")).ok is True
    _patch_httpx(monkeypatch, _resp(403, {}))
    assert asyncio.run(A._validate_gemini("k")).ok is False
    _patch_httpx(monkeypatch, _resp(400, {"error": {"details": [{"reason": "API_KEY_INVALID"}]}}))
    assert asyncio.run(A._validate_gemini("k")).ok is False
    _patch_httpx(monkeypatch, exc=RuntimeError("boom"))
    assert asyncio.run(A._validate_claude("k")).ok is False
    _patch_httpx(monkeypatch, _resp(500, {}))
    assert asyncio.run(A._validate_claude("k")).ok is False
    # gemini fall-through (non-200/401/403, not an invalid-key 400)
    _patch_httpx(monkeypatch, _resp(500, {"error": {"message": "server"}}))
    assert asyncio.run(A._validate_gemini("k")).ok is False
    # claude 403 and 400-invalid-key
    _patch_httpx(monkeypatch, _resp(403, {}))
    assert asyncio.run(A._validate_claude("k")).ok is False
    _patch_httpx(monkeypatch, _resp(400, {"error": {"message": "invalid api key"}}))
    assert asyncio.run(A._validate_claude("k")).ok is False


# --------------------------------------------------------------------------- #
# routes (auth dependencies overridden to no-ops)
# --------------------------------------------------------------------------- #
@pytest.fixture()
def client(monkeypatch):
    monkeypatch.setattr(A, "require_scope", lambda *a, **k: (lambda: None))
    monkeypatch.setattr(A, "require_service_token", lambda: None)
    return TestClient(A.create_app())


def test_status_routes(client):
    from constants import STATUS_LIVE_PATH, STATUS_READY_PATH

    assert client.get(STATUS_LIVE_PATH).json()["status"]
    assert client.get(STATUS_READY_PATH).json()["status"]


def test_validate_route_branches(client, monkeypatch):
    from api_server import _ValidationResult

    async def ok(*a, **k):
        return _ValidationResult(ok=True)

    async def bad(*a, **k):
        return _ValidationResult(ok=False, reason="nope")

    # empty key
    r = client.post("/provider/validate-api-key", json={"provider": "groq", "api_key": ""})
    assert r.json()["ok"] is False
    # ollama not allowed
    r = client.post("/provider/validate-api-key", json={"provider": "ollama", "api_key": "x"})
    assert r.json()["ok"] is False
    # unknown provider
    r = client.post("/provider/validate-api-key", json={"provider": "mystery", "api_key": "x"})
    assert r.json()["ok"] is False
    # gemini ok
    monkeypatch.setattr(A, "_validate_gemini", ok)
    assert client.post("/provider/validate-api-key", json={"provider": "gemini", "api_key": "x"}).json()["ok"] is True
    # groq/chatgpt via models list
    monkeypatch.setattr(A, "_validate_models_list", ok)
    assert client.post("/provider/validate-api-key", json={"provider": "groq", "api_key": "x"}).json()["ok"] is True
    assert client.post("/provider/validate-api-key", json={"provider": "chatgpt", "api_key": "x"}).json()["ok"] is True
    # claude failing path
    monkeypatch.setattr(A, "_validate_claude", bad)
    r = client.post("/provider/validate-api-key", json={"provider": "claude", "api_key": "x"})
    assert r.json()["ok"] is False and r.json()["reason"] == "nope"


def test_dispatch_route(client, monkeypatch):
    monkeypatch.setattr(A, "dispatch_applications", lambda items: (["a"], ["b"]))
    r = client.post("/api/v1/insights/applications", json={"items": []})
    body = r.json()
    assert body["operation"] and body["data"]["ready"] == ["a"] and body["data"]["pending"] == ["b"]


def test_run_api_in_thread(monkeypatch):
    served = {}
    fake_uv = types.ModuleType("uvicorn")
    fake_uv.Config = lambda *a, **k: object()
    fake_uv.Server = lambda config: types.SimpleNamespace(serve=lambda: None)
    monkeypatch.setitem(sys.modules, "uvicorn", fake_uv)
    monkeypatch.setattr(A.asyncio, "run", lambda coro: served.setdefault("ran", True))
    A.run_api_in_thread()
    assert served["ran"] is True
