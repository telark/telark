"""Coverage for authz.py — permission resolution, scope dependency.

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


# ---- _resolve --------------------------------------------------------------
def test_resolve_401(monkeypatch):
    _patch_resp(monkeypatch, _resp(401))
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ._resolve("t"))
    assert e.value.status_code == 401
    assert e.value.detail == AZ.MSG_AUTHZ_INVALID_SESSION != AZ.MSG_AUTHZ_MISSING_SESSION


def test_resolve_403_is_forbidden(monkeypatch):
    # auth-service answers 403 for a suspended or deleted user: a verdict, not an outage.
    _patch_resp(monkeypatch, _resp(403))
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ._resolve("t"))
    assert e.value.status_code == 403
    assert e.value.detail == AZ.MSG_AUTHZ_FORBIDDEN


def test_resolve_ok(monkeypatch):
    _patch_resp(monkeypatch, _resp(200, {"userID": "u", "roles": [{"r": 1}]}))
    assert asyncio.run(AZ._resolve("t")) == ("u", [{"r": 1}])
    _patch_resp(monkeypatch, _resp(200, {}))
    assert asyncio.run(AZ._resolve("t")) == ("", [])


def test_resolve_500_raises(monkeypatch):
    _patch_resp(monkeypatch, _resp(500))
    with pytest.raises(httpx.HTTPStatusError):
        asyncio.run(AZ._resolve("t"))


# ---- require_any dependency -----------------------------------------------
def test_scope_missing_token():
    dep = AZ.require_any(("settings", "Admin", None))
    with pytest.raises(HTTPException) as e:
        asyncio.run(dep(session_token=None))
    assert e.value.status_code == 401


def test_scope_resolve_http_exc_reraised(monkeypatch):
    async def boom(_):
        raise HTTPException(401, "x")

    monkeypatch.setattr(AZ, "_resolve", boom)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_any(("s", "Admin", None))(session_token="tok"))
    assert e.value.status_code == 401


def test_scope_resolve_generic_is_503(monkeypatch):
    async def boom(_):
        raise RuntimeError("auth down")

    monkeypatch.setattr(AZ, "_resolve", boom)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_any(("s", "Admin", None))(session_token="tok"))
    assert e.value.status_code == 503


def test_scope_forbidden(monkeypatch):
    async def roles(_):
        return "u", []

    monkeypatch.setattr(AZ, "_resolve", roles)
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_any(("s", "Admin", None))(session_token="tok"))
    assert e.value.status_code == 403


def test_scope_granted(monkeypatch):
    async def roles(_):
        return "u1", [{"status": "Active", "scopes": [{"scope": "s", "level": "Admin"}]}]

    monkeypatch.setattr(AZ, "_resolve", roles)
    assert asyncio.run(AZ.require_any(("s", "Admin", None))(session_token="tok")) == "u1"


def test_scope_action_rule_denies(monkeypatch):
    async def roles(_):
        return "u", [{"status": "Active", "scopes": [
            {"scope": "settings", "level": "Owner", "rules": ["settings.controlainsights.deny"]}]}]

    monkeypatch.setattr(AZ, "_resolve", roles)
    assert asyncio.run(AZ.require_any(("settings", "Owner", None))(session_token="tok")) == "u"
    with pytest.raises(HTTPException) as e:
        asyncio.run(AZ.require_any(("settings", "Owner", "controlainsights"))(session_token="tok"))
    assert e.value.status_code == 403


def test_scope_action_granted_without_rule(monkeypatch):
    async def roles(_):
        return "u", [{"status": "Active", "scopes": [{"scope": "ALL", "level": "Admin"}]}]

    monkeypatch.setattr(AZ, "_resolve", roles)
    assert asyncio.run(AZ.require_any(("settings", "Owner", "controlainsights"))(session_token="tok")) == "u"


def test_any_alternative_admits(monkeypatch):
    async def roles(_):
        return "u", [{"status": "Active", "scopes": [{"scope": "settings", "level": "Owner"}]}]

    monkeypatch.setattr(AZ, "_resolve", roles)
    either = AZ.require_any(("insights", "ReadOnly", None), ("settings", "Owner", None))
    assert asyncio.run(either(session_token="tok")) == "u"
    neither = AZ.require_any(("insights", "ReadOnly", None), ("settings", "Admin", None))
    with pytest.raises(HTTPException) as e:
        asyncio.run(neither(session_token="tok"))
    assert e.value.status_code == 403
