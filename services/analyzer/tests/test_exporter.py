"""Checks for the exporter client: the TelarkConfig AI read and the application GET.

This decides whether the analyzer runs at all and which model it asks, so a
parse slip silently disables it or points it at the wrong model.

Run: python -m pytest tests/test_exporter.py
"""

import asyncio
import os
import sys

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import exporter  # noqa: E402
from constants import DEFAULT_ANALYZER_MODEL, HEADER_SERVICE_TOKEN  # noqa: E402
from models import AnalyzerConfig  # noqa: E402


def _envelope(data):
    return {"status": 200, "operation": "Success", "data": data}


def _call(handler, fn):
    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(handler)) as client:
            return await fn(client)

    return asyncio.run(go())


def _config_handler(data):
    return lambda request: httpx.Response(200, json=_envelope(data))


@pytest.fixture(autouse=True)
def _never_loaded(monkeypatch):
    monkeypatch.setattr(exporter, "_current", AnalyzerConfig())


def test_fetch_config_parses_ai_and_excluded():
    handler = _config_handler({
        "ai": {"enabled": True, "model": "qwen2.5:7b", "autoAnalyze": True},
        "excludedNamespaces": ["kube-system", "telark"],
    })
    cfg = _call(handler, exporter.fetch_config)
    assert cfg.enabled is True
    assert cfg.model == "qwen2.5:7b"
    assert cfg.autoAnalyze is True
    assert cfg.excludedNamespaces == ["kube-system", "telark"]


def test_empty_model_uses_default():
    # An analyzer image newer than the CRD sees spec.ai.model pruned.
    cfg = _call(_config_handler({"ai": {"enabled": True, "model": ""}}), exporter.fetch_config)
    assert cfg.model == DEFAULT_ANALYZER_MODEL
    cfg = _call(_config_handler({"ai": {"enabled": True}}), exporter.fetch_config)
    assert cfg.model == DEFAULT_ANALYZER_MODEL
    assert cfg.autoAnalyze is False and cfg.excludedNamespaces == []


def test_failed_refresh_keeps_last_good():
    _call(_config_handler({"ai": {"enabled": True, "model": "qwen3:4b"}}), exporter.refresh)
    assert exporter.current().enabled is True

    def down(request):
        raise httpx.ConnectError("exporter unreachable")

    _call(down, exporter.refresh)
    assert exporter.current().enabled is True, "a transient failure must not disable a working analyzer"
    _call(lambda request: httpx.Response(500), exporter.refresh)
    assert exporter.current().enabled is True


def test_never_loaded_is_disabled():
    assert exporter.current() == AnalyzerConfig()
    assert exporter.current().enabled is False
    _call(lambda request: httpx.Response(503), exporter.refresh)
    assert exporter.current().enabled is False, "no config yet + failed read must be disabled, not a guess"


def test_service_token_sent(monkeypatch):
    monkeypatch.setattr(exporter, "SERVICE_TOKEN", "svc-token")
    seen = []

    def handler(request):
        seen.append(request.headers.get(HEADER_SERVICE_TOKEN))
        return httpx.Response(200, json=_envelope({"name": "api"}))

    _call(handler, exporter.fetch_config)
    _call(handler, lambda client: exporter.get_application(client, "api"))
    assert seen == ["svc-token", "svc-token"]


def test_get_application_percent_encodes_the_name():
    seen = []

    def handler(request):
        seen.append(request.url.raw_path)
        return httpx.Response(200, json=_envelope({}))

    _call(handler, lambda client: exporter.get_application(client, "foo/get?x=1"))
    assert seen == [b"/api/v1/applications/foo%2Fget%3Fx%3D1"]


def test_get_application_returns_data():
    def handler(request):
        assert request.url.path == "/api/v1/applications/api"
        return httpx.Response(200, json=_envelope({"name": "api"}))

    assert _call(handler, lambda client: exporter.get_application(client, "api")) == {"name": "api"}


def test_get_application_not_found():
    with pytest.raises(exporter.AppNotFound):
        _call(lambda request: httpx.Response(404), lambda client: exporter.get_application(client, "gone"))


def test_get_application_unavailable():
    with pytest.raises(exporter.ExporterUnavailable):
        _call(lambda request: httpx.Response(500), lambda client: exporter.get_application(client, "api"))

    def down(request):
        raise httpx.ConnectError("refused")

    with pytest.raises(exporter.ExporterUnavailable):
        _call(down, lambda client: exporter.get_application(client, "api"))


# ---- review lists: apps (summary view), plans, plan environments (S7) ----------------------------
def _list_handler(seen, items_by_path):
    def handler(request):
        seen.append((request.url.path, dict(request.url.params), request.headers.get(HEADER_SERVICE_TOKEN)))
        items = items_by_path.get(request.url.path)
        if items is None:
            return httpx.Response(503)
        return httpx.Response(200, json=_envelope({"items": items}))
    return handler


def test_list_applications_summary_view(monkeypatch):
    monkeypatch.setattr(exporter, "SERVICE_TOKEN", "svc-token")
    seen = []
    apps = [{"name": "shop", "history": {"generation": 3}}]
    handler = _list_handler(seen, {"/api/v1/applications": apps})
    assert _call(handler, exporter.list_applications) == apps
    assert seen == [("/api/v1/applications", {"view": "summary"}, "svc-token")]
    # Any failure is ExporterUnavailable: a status, a transport error, a body without items.
    for bad in (lambda r: httpx.Response(500), lambda r: httpx.Response(200, json=_envelope({"x": 1})),
                lambda r: httpx.Response(200, text="not json")):
        with pytest.raises(exporter.ExporterUnavailable):
            _call(bad, exporter.list_applications)

    def down(request):
        raise httpx.ReadTimeout("slow")
    with pytest.raises(exporter.ExporterUnavailable):
        _call(down, exporter.list_applications)


def test_list_plans_and_environments_service_token(monkeypatch):
    monkeypatch.setattr(exporter, "SERVICE_TOKEN", "svc-token")
    seen = []
    handler = _list_handler(seen, {
        "/api/v1/protectionplans": [{"id": "p1", "phase": "active"}],
        "/api/v1/categories": [
            {"id": "e1", "name": "Production"}, {"id": "e2"}],
    })
    assert _call(handler, exporter.list_plans) == [{"id": "p1", "phase": "active"}]
    assert _call(handler, exporter.list_environments) == {"e1": "Production", "e2": ""}
    assert [token for _, _, token in seen] == ["svc-token", "svc-token"]
    assert seen[1][:2] == ("/api/v1/categories", {"scope": "plan-environments"})


def test_plans_snapshot_cached_for_tick(monkeypatch):
    monkeypatch.setattr(exporter, "_plans_cache", None)
    seen = []
    items = {"/api/v1/protectionplans": [{"id": "p1"}],
             "/api/v1/categories": [{"id": "e1", "name": "prod"}]}
    handler = _list_handler(seen, items)
    first = _call(handler, lambda c: exporter.plans_snapshot(c, 1000.0))
    again = _call(handler, lambda c: exporter.plans_snapshot(c, 1000.0 + exporter.ANALYZER_REVIEW_TICK_SEC - 1))
    assert first == again == ([{"id": "p1"}], {"e1": "prod"}) and len(seen) == 2
    later = 1000.0 + exporter.ANALYZER_REVIEW_TICK_SEC
    _call(handler, lambda c: exporter.plans_snapshot(c, later))
    assert len(seen) == 4
    # A failure is never cached: the next review reads again.
    items.pop("/api/v1/protectionplans")
    with pytest.raises(exporter.ExporterUnavailable):
        _call(handler, lambda c: exporter.plans_snapshot(c, later + 10_000))
    assert exporter._plans_cache[0] == later
