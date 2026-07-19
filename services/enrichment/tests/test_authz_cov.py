"""Coverage for authz.py — service token, permission resolution, scope dependency.

Run: pytest tests/test_authz_cov.py
"""

import asyncio
import os
import sys

import httpx
import pytest
from fastapi import HTTPException

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import authz as AZ  # noqa: E402


def _resp(status, payload=None):
    return httpx.Response(status, json=payload or {}, request=httpx.Request("GET", "https://x"))


class _FakeAC:
    def __init__(self, resp):
        self._resp = resp

    async def __aenter__(self):
        return self

    async def __aexit__(self, *a):
        return False

    async def get(self, *a, **k):
        return self._resp


def _patch_resp(monkeypatch, resp):
    monkeypatch.setattr(AZ.httpx, "AsyncClient", lambda *a, **k: _FakeAC(resp))


# ---- service token ---------------------------------------------------------
def test_service_token_accept_and_reject(monkeypatch):
    monkeypatch.setattr(AZ, "SERVICE_TOKEN", "secret")
    assert AZ.require_service_token("secret") is None
    with pytest.raises(HTTPException):
        AZ.require_service_token("wrong")
    with pytest.raises(HTTPException):
        AZ.require_service_token(None)


def test_service_token_unset_denies(monkeypatch):
    monkeypatch.setattr(AZ, "SERVICE_TOKEN", "")
    with pytest.raises(HTTPException):
        AZ.require_service_token("anything")


# ---- _resolve --------------------------------------------------------------
def test_resolve_401(monkeypatch):
    _patch_resp(monkeypatch, _resp(401))
    with pytest.raises(HTTPException):
        asyncio.run(AZ._resolve("t"))


def test_resolve_ok(monkeypatch):
    _patch_resp(monkeypatch, _resp(200, {"data": {"roles": [{"r": 1}]}}))
    assert asyncio.run(AZ._resolve("t")) == [{"r": 1}]


def test_resolve_500_raises(monkeypatch):
    _patch_resp(monkeypatch, _resp(500))
    with pytest.raises(httpx.HTTPStatusError):
        asyncio.run(AZ._resolve("t"))


# ---- require_scope dependency ---------------------------------------------
def test_scope_missing_token():
    dep = AZ.require_scope("settings", "Admin")
    with pytest.raises(HTTPException) as e:
        asyncio.run(dep(session_token=None))
    assert e.value.status_code == 401


def test_scope_resolve_http_exc_reraised(monkeypatch):
    async def boom(_):
        raise HTTPException(401, "x")

    monkeypatch.setattr(AZ, "_resolve", boom)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_scope("s", "Admin")(session_token="tok"))
    assert e.value.status_code == 401


def test_scope_resolve_generic_is_503(monkeypatch):
    async def boom(_):
        raise RuntimeError("auth down")

    monkeypatch.setattr(AZ, "_resolve", boom)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_scope("s", "Admin")(session_token="tok"))
    assert e.value.status_code == 503


def test_scope_forbidden(monkeypatch):
    async def roles(_):
        return []

    monkeypatch.setattr(AZ, "_resolve", roles)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_scope("s", "Admin")(session_token="tok"))
    assert e.value.status_code == 403


def test_scope_granted(monkeypatch):
    async def roles(_):
        return [{"status": "Active", "scopes": [{"scope": "s", "level": "Admin"}]}]

    monkeypatch.setattr(AZ, "_resolve", roles)
    assert asyncio.run(AZ.require_scope("s", "Admin")(session_token="tok")) is None
