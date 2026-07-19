"""Full coverage for the provider layer — shared InstructorProvider flow, the
config-only subclasses, Ollama's raise-on-unavailable policy, the Anthropic
free-text path, and the factory.

Run: pytest tests/test_providers_cov.py
"""

import os
import sys

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from datetime import datetime  # noqa: E402

from models import EnrichmentResult, EnrichmentResultLLM  # noqa: E402
from providers import cache as C  # noqa: E402
import providers as PKG  # noqa: E402
from providers.anthropic import AnthropicProvider, _strip_fences  # noqa: E402
from providers.base import InstructorProvider, fallback_result  # noqa: E402
from providers.gemini import GeminiProvider  # noqa: E402
from providers.groq import GroqProvider  # noqa: E402
from providers.ollama import OllamaProvider, OllamaUnavailableError  # noqa: E402
from models import AppSignals  # noqa: E402


LLM_JSON = {
    "summary": "does x", "techStack": ["Redis"], "role": "backend-service",
    "dependencies": ["redis"], "confidence": "high", "category": "application",
    "risks": [{"severity": "high", "message": "r"}],
    "suggestions": [{"priority": "low", "message": "s"}],
    "resourceEfficiency": {"status": "under", "note": "n"},
    "criticality": {"level": "high", "reason": "why"}, "tags": ["t"],
    "relatedApps": [{"name": "redis", "reason": "REDIS_HOST"}],
}


def _llm():
    return EnrichmentResultLLM.model_validate(LLM_JSON)


class _FakeCompletions:
    def __init__(self, behavior):
        self.behavior = list(behavior)
        self.calls = 0

    def create(self, **kw):
        b = self.behavior[min(self.calls, len(self.behavior) - 1)]
        self.calls += 1
        if isinstance(b, Exception):
            raise b
        return b


class _FakeClient:
    def __init__(self, behavior):
        self.chat = type("Chat", (), {"completions": _FakeCompletions(behavior)})()


def _sig():
    return AppSignals(name="a", namespace="ns", images=["nginx"], ports=[80], envVarKeys=["X"])


def _patched(provider, behavior):
    provider._client = lambda: _FakeClient(behavior)
    return provider


def setup_function(_):
    C._RESPONSE_CACHE.clear()


# ---- fallback + subclass wiring -------------------------------------------
def test_fallback_result():
    r = fallback_result()
    assert r.role and r.confidence == "low" and r.resourceEfficiency.status == "unknown"


def test_subclass_attrs():
    assert GroqProvider.name == "groq" and GroqProvider.model
    assert GeminiProvider.name == "gemini" and GeminiProvider.base_url
    assert OllamaProvider.name == "ollama" and OllamaProvider.api_key_required is False


def test_base_resolve_defaults():
    p = GroqProvider("mykey")
    assert p._resolve_base_url() == GroqProvider.base_url
    assert p._resolve_key() == "mykey"


def test_client_builds_real_instructor():
    # OpenAI/instructor client construction is lazy — no network until a call.
    client = GroqProvider("offline-key")._client()
    assert client is not None


def test_enrich_loop_exhausts_to_fallback():
    # _handle_error that never stops and never raises drives the loop to completion,
    # hitting the trailing fallback return.
    class NeverStops(InstructorProvider):
        name, model, retries, api_key_required = "never", "m", 1, False

        def _handle_error(self, e, attempt):
            return None

    p = NeverStops("")
    p._client = lambda: _FakeClient([Exception("x")])
    assert p.enrich(_sig()).summary == fallback_result().summary


# ---- shared InstructorProvider flow ---------------------------------------
def test_cache_hit_short_circuits():
    p = _patched(GroqProvider("k"), [RuntimeError("must not be called")])
    key = C.cache_key(C.signals_hash(_sig()))
    cached = EnrichmentResult(summary="cached", role="r", confidence="low", enrichedAt=datetime.utcnow())
    C.set_cached(key, cached)
    assert p.enrich(_sig()) is cached


def test_success_maps_and_caches():
    p = _patched(GroqProvider("k"), [_llm()])
    r = p.enrich(_sig())
    assert r.summary == "does x" and r.criticality.level == "high"
    # second call hits cache (create would raise if invoked again)
    p._client = lambda: _FakeClient([RuntimeError("no")])
    assert p.enrich(_sig()).summary == "does x"


def test_retry_then_success():
    p = _patched(GroqProvider("k"), [Exception("transient blip"), _llm()])
    r = p.enrich(_sig())
    assert r.summary == "does x"


def test_auth_error_falls_back():
    p = _patched(GroqProvider("k"), [Exception("authentication failed")])
    assert p.enrich(_sig()).summary == fallback_result().summary


def test_rate_error_falls_back():
    p = _patched(GroqProvider("k"), [Exception("rate limit exceeded")])
    assert p.enrich(_sig()).summary == fallback_result().summary


def test_retries_exhausted_falls_back():
    p = _patched(GroqProvider("k"), [Exception("weird") for _ in range(GroqProvider.retries)])
    assert p.enrich(_sig()).summary == fallback_result().summary


def test_setup_error_no_key_falls_back():
    # real _client() runs and raises ValueError (no key) -> _on_setup_error
    assert GroqProvider("").enrich(_sig()).summary == fallback_result().summary


# ---- Ollama overrides ------------------------------------------------------
def test_ollama_base_url_and_key():
    p = OllamaProvider()
    assert p._resolve_base_url().endswith("/v1")
    assert p._resolve_key() == "ollama"


def test_ollama_setup_error_raises():
    with pytest.raises(OllamaUnavailableError):
        OllamaProvider()._on_setup_error(Exception("down"))


def test_ollama_handle_error_policy():
    p = OllamaProvider()
    with pytest.raises(OllamaUnavailableError):
        p._handle_error(Exception("connection refused"), attempt=1)
    # non-connection, mid-retry -> None (keep retrying)
    assert p._handle_error(Exception("blip"), attempt=1) is None
    # exhausted -> fallback
    assert p._handle_error(Exception("blip"), attempt=p.retries).summary == fallback_result().summary


def test_ollama_enrich_success():
    p = _patched(OllamaProvider(), [_llm()])
    assert p.enrich(_sig()).summary == "does x"


def test_ollama_enrich_connection_raises():
    p = _patched(OllamaProvider(), [Exception("connection refused")])
    with pytest.raises(OllamaUnavailableError):
        p.enrich(_sig())


# ---- Anthropic -------------------------------------------------------------
def test_strip_fences():
    assert _strip_fences("{\"a\":1}") == '{"a":1}'
    assert _strip_fences("```json\n{\"a\":1}\n```") == '{"a":1}'
    assert _strip_fences("```\n{\"a\":1}\n```") == '{"a":1}'


class _Msg:
    def __init__(self, text):
        self.content = [type("Blk", (), {"text": text})()]


def _fake_anthropic(monkeypatch, behavior):
    import json as _json
    import providers.anthropic as A

    class FakeMessages:
        def create(self, **kw):
            if isinstance(behavior, Exception):
                raise behavior
            return _Msg(behavior)

    class FakeClient:
        def __init__(self, api_key=None):
            self.messages = FakeMessages()

    monkeypatch.setattr(A.anthropic, "Anthropic", FakeClient)
    return _json


def test_anthropic_no_key_raises():
    with pytest.raises(ValueError):
        AnthropicProvider("").enrich(_sig())


def test_anthropic_success(monkeypatch):
    import json
    _fake_anthropic(monkeypatch, "```json\n" + json.dumps(LLM_JSON) + "\n```")
    r = AnthropicProvider("k").enrich(_sig())
    assert r.summary == "does x" and r.tags == ["t"]


def test_anthropic_cache_hit(monkeypatch):
    _fake_anthropic(monkeypatch, "irrelevant")
    key = C.cache_key(C.signals_hash(_sig()))
    cached = EnrichmentResult(summary="cached", role="r", confidence="low", enrichedAt=datetime.utcnow())
    C.set_cached(key, cached)
    assert AnthropicProvider("k").enrich(_sig()) is cached


def test_anthropic_parse_error_falls_back(monkeypatch):
    _fake_anthropic(monkeypatch, "not json at all")
    assert AnthropicProvider("k").enrich(_sig()).summary == fallback_result().summary


def test_anthropic_connection_error_raises(monkeypatch):
    import anthropic
    req = httpx.Request("POST", "https://api.anthropic.com")
    _fake_anthropic(monkeypatch, anthropic.APIConnectionError(request=req))
    with pytest.raises(anthropic.APIConnectionError):
        AnthropicProvider("k").enrich(_sig())


def test_anthropic_auth_error_raises(monkeypatch):
    import anthropic
    req = httpx.Request("POST", "https://api.anthropic.com")
    resp = httpx.Response(401, request=req)
    err = anthropic.AuthenticationError("bad key", response=resp, body=None)
    _fake_anthropic(monkeypatch, err)
    with pytest.raises(anthropic.AuthenticationError):
        AnthropicProvider("k").enrich(_sig())


def test_anthropic_generic_error_falls_back(monkeypatch):
    _fake_anthropic(monkeypatch, RuntimeError("boom"))
    assert AnthropicProvider("k").enrich(_sig()).summary == fallback_result().summary


# ---- factory ---------------------------------------------------------------
def _cfg(enabled=True, provider="groq", api_key="k"):
    return type("Cfg", (), {"enabled": enabled, "provider": provider, "api_key": api_key})()


def _reset_factory():
    PKG._cache_key = None
    PKG._provider = None


def test_build_each_and_unknown(monkeypatch):
    assert isinstance(PKG._build("anthropic", "k"), AnthropicProvider)
    assert isinstance(PKG._build("groq", "k"), GroqProvider)
    assert isinstance(PKG._build("gemini", "k"), GeminiProvider)
    assert isinstance(PKG._build("ollama", ""), OllamaProvider)
    assert PKG._build("mystery", "k") is None


def test_get_provider_disabled(monkeypatch):
    _reset_factory()
    monkeypatch.setattr(PKG, "get_ai_config", lambda: _cfg(enabled=False))
    assert PKG.get_provider() is None


def test_get_provider_cloud_without_key(monkeypatch):
    _reset_factory()
    monkeypatch.setattr(PKG, "get_ai_config", lambda: _cfg(provider="groq", api_key=""))
    assert PKG.get_provider() is None


def test_get_provider_ollama_and_memoization(monkeypatch):
    _reset_factory()
    calls = {"n": 0}

    def cfg():
        calls["n"] += 1
        return _cfg(provider="ollama", api_key="")

    monkeypatch.setattr(PKG, "get_ai_config", cfg)
    p1 = PKG.get_provider()
    p2 = PKG.get_provider()
    assert isinstance(p1, OllamaProvider) and p1 is p2  # memoised on (provider, key)
    _reset_factory()
