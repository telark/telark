"""Checks for the AI config read from GlobalConfig.

This is the security path: which key enrichment calls the LLM with comes from
here. The heavy runtime deps (httpx, loguru, dotenv) are stubbed so the real
parsing, caching, and fallback logic runs without them installed.

Run: python3 tests/test_provider_config.py
"""

import os
import sys
import types

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

# --- stub the imports provider_config pulls in, before importing it ----------
_httpx = types.ModuleType("httpx")
_httpx.get = lambda *a, **k: None  # replaced per test
sys.modules["httpx"] = _httpx

_dotenv = types.ModuleType("dotenv")
_dotenv.load_dotenv = lambda *a, **k: None
sys.modules["dotenv"] = _dotenv

_app_logger = types.ModuleType("app_logger")
_app_logger.logger = types.SimpleNamespace(
    warning=lambda *a, **k: None,
    info=lambda *a, **k: None,
    error=lambda *a, **k: None,
)
sys.modules["app_logger"] = _app_logger

import provider_config as pc  # noqa: E402


class _Resp:
    def __init__(self, payload):
        self._payload = payload

    def raise_for_status(self):
        pass

    def json(self):
        return self._payload


def _reset():
    pc._cached = None
    pc._fetched_at = 0.0


def _set_response(payload):
    _httpx.get = lambda *a, **k: _Resp(payload)


def _set_failure():
    def _boom(*a, **k):
        raise RuntimeError("exporter unreachable")

    _httpx.get = _boom


def test_parses_ai_block():
    _reset()
    _set_response({"data": {"ai": {"enabled": True, "provider": "Gemini", "apiKey": "sk-1"}}})
    cfg = pc.get_ai_config()
    assert cfg.enabled is True, cfg
    assert cfg.provider == "gemini", cfg  # normalised to lowercase
    assert cfg.api_key == "sk-1", cfg


def test_missing_ai_block_is_disabled():
    _reset()
    _set_response({"data": {}})
    cfg = pc.get_ai_config()
    assert cfg.enabled is False and cfg.provider == "" and cfg.api_key == "", cfg


def test_caches_within_ttl():
    _reset()
    calls = {"n": 0}

    def _counting(*a, **k):
        calls["n"] += 1
        return _Resp({"data": {"ai": {"enabled": True, "provider": "groq", "apiKey": "k"}}})

    _httpx.get = _counting
    pc.get_ai_config()
    pc.get_ai_config()
    assert calls["n"] == 1, f"expected one fetch within TTL, got {calls['n']}"


def test_failure_keeps_last_good():
    _reset()
    _set_response({"data": {"ai": {"enabled": True, "provider": "groq", "apiKey": "good"}}})
    first = pc.get_ai_config()
    assert first.api_key == "good"

    pc._fetched_at = 0.0  # force the TTL to be considered expired
    _set_failure()
    second = pc.get_ai_config()
    assert second.api_key == "good", "a transient failure must not drop a working key"


def test_failure_with_no_prior_is_disabled():
    _reset()
    _set_failure()
    cfg = pc.get_ai_config()
    assert cfg.enabled is False, "no config yet + fetch failure must be disabled, not a guess"


def _run():
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            fn()
            print(f"ok  {name}")
    print("all provider_config tests passed")


if __name__ == "__main__":
    _run()
