"""Coverage for api_server.py: status probes, the analyzer routes and the SSE stream.

Run: pytest tests/test_api_server_cov.py
"""

import asyncio
import os
import sys
from types import SimpleNamespace

import httpx
import pytest
from fastapi import HTTPException
from fastapi.responses import StreamingResponse
from fastapi.routing import APIRoute
from fastapi.testclient import TestClient
from redis import ConnectionError as RedisConnectionError
from redis import RedisError

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import api_server as A  # noqa: E402
import authz  # noqa: E402
import exporter  # noqa: E402
from constants import (  # noqa: E402
    ANALYZE_PATH,
    EVENTS_PATH,
    RUNTIME_PATH,
    RUNTIME_PULL_PATH,
    RUNTIME_VALIDATE_PATH,
    STATUS_LIVE_PATH,
    STATUS_READY_PATH,
    TRIAGE_PATH,
)
from events import Broadcaster  # noqa: E402
from exporter import AppNotFound, ExporterUnavailable  # noqa: E402
from fakes import FakeOllama, FakeRedis, dns_error  # noqa: E402
from insights import InsightStore  # noqa: E402
from models import AnalyzerConfig, AppInsights, Insight  # noqa: E402
from runtime import Runtime  # noqa: E402

NS, NAME = "shop", "api"
ANALYZE = "/api/v1/insights/applications/shop/api/analyze"
EVENTS = "/api/v1/insights/events"
RUNTIME = "/api/v1/insights/runtime"
VALIDATE = "/api/v1/insights/runtime/validate"
PULL = "/api/v1/insights/runtime/pull"
STREAM = "insights:jobs"
APP = {"name": NAME, "namespaces": {"items": [{"name": NS}]}, "history": {"generation": 7}}


def _allow():
    return None


def _env(monkeypatch, *, enabled=True, excluded=(), app=None, state="ready", mode="deep"):
    fake = FakeRedis()
    api = A.create_app(fake)
    s = api.state
    s.store = InsightStore(fake)
    s.broadcaster = Broadcaster()
    s.exporter_client = None
    ollama = FakeOllama()
    s.runtime = Runtime(ollama.client(), s.broadcaster, mode=mode)
    s.runtime.status.state = state
    s.runtime.status.model = "qwen3:4b"
    monkeypatch.setattr(exporter, "_current", AnalyzerConfig(enabled=enabled, excludedNamespaces=list(excluded)))
    calls = []

    async def get_application(client, name):
        calls.append(name)
        if isinstance(app, Exception):
            raise app
        return APP if app is None else app

    monkeypatch.setattr(exporter, "get_application", get_application)
    for route in api.routes:
        for dep in getattr(route, "dependencies", []):
            api.dependency_overrides[dep.dependency] = _allow
    return SimpleNamespace(api=api, fake=fake, state=s, ollama=ollama, client=TestClient(api), exporter_calls=calls)


def _code(r):
    return r.json()["data"]["code"]


def _drain(sub):
    out = []
    while not sub.queue.empty():
        out.append(sub.queue.get_nowait())
    return out


def _doc(env) -> AppInsights:
    return asyncio.run(env.state.store.get(NS, NAME))


def _xlen(env) -> int:
    return asyncio.run(env.fake.xlen(STREAM))


def _endpoint(api, path):
    return next(r.endpoint for r in api.routes if getattr(r, "path", "") == path)


# ---- probes ------------------------------------------------------------------
def test_live_always_200():
    fake = FakeRedis()
    fake.ping_error = RedisConnectionError("down")
    r = TestClient(A.create_app(fake)).get(STATUS_LIVE_PATH)
    assert r.status_code == 200 and r.json() == {"status": "alive", "service": "analyzer-service"}


def test_ready_200_when_redis_answers():
    r = TestClient(A.create_app(FakeRedis())).get(STATUS_READY_PATH)
    assert r.status_code == 200 and r.json() == {"status": "ready", "service": "analyzer-service"}


def test_ready_503_when_ping_fails():
    fake = FakeRedis()
    fake.ping_error = RedisConnectionError("down")
    r = TestClient(A.create_app(fake)).get(STATUS_READY_PATH)
    assert r.status_code == 503 and r.json()["status"] == "not_ready"


def test_ready_503_when_ping_hangs(monkeypatch):
    class Hanging:
        async def ping(self):
            await asyncio.sleep(1)

    monkeypatch.setattr(A, "READY_PING_TIMEOUT_S", 0.01)
    assert TestClient(A.create_app(Hanging())).get(STATUS_READY_PATH).status_code == 503


# ---- route requirements --------------------------------------------------------
def test_route_requirements(monkeypatch):
    def recording_any(*requirements):
        async def dependency():
            return None

        dependency.requirement = requirements
        return dependency

    monkeypatch.setattr(A, "require_any", recording_any)
    api = A.create_app(FakeRedis())
    got = {}
    for route in api.routes:
        if not isinstance(route, APIRoute):
            continue
        if route.path in (STATUS_LIVE_PATH, STATUS_READY_PATH):
            assert route.dependencies == []
            continue
        (dep,) = route.dependencies
        for method in route.methods:
            got[(method, route.path)] = dep.dependency.requirement

    read = (("insights", "ReadOnly", None), ("settings", "Owner", None))
    want = {
        ("POST", "/api/v1/insights/applications/{namespace}/{name}/analyze"): (
            ("insights", "Contributor", "analyzeinsights"),),
        ("GET", "/api/v1/insights/events"): read,
        ("GET", "/api/v1/insights/runtime"): read,
        ("POST", "/api/v1/insights/runtime/validate"): (("settings", "Owner", "controlainsights"),),
        ("POST", "/api/v1/insights/runtime/pull"): (("settings", "Owner", "controlainsights"),),
        ("POST", "/api/v1/insights/applications/{namespace}/{name}/insights/{id}/triage"): (
            ("insights", "Contributor", "triageinsights"),),
    }
    assert got == want == A.ROUTE_REQUIREMENTS
    assert not any("token" in getattr(route, "path", "").lower() for route in api.routes)


def test_insights_scope_matrix(monkeypatch):
    guards = A.create_app(FakeRedis()).state.guards
    analyze, triage = ("POST", ANALYZE_PATH), ("POST", TRIAGE_PATH)
    events, runtime = ("GET", EVENTS_PATH), ("GET", RUNTIME_PATH)
    validate, pull = ("POST", RUNTIME_VALIDATE_PATH), ("POST", RUNTIME_PULL_PATH)
    no_analyze, no_triage = ["insights.analyzeinsights.deny"], ["insights.triageinsights.deny"]
    cases = [
        ("applications only", ("applications", "Admin", []), set()),
        ("insights reader", ("insights", "ReadOnly", []), {events, runtime}),
        ("insights contributor", ("insights", "Contributor", []), {analyze, triage, events, runtime}),
        ("settings owner", ("settings", "Owner", []), {events, runtime, validate, pull}),
        ("admin via ALL", ("ALL", "Admin", []), set(guards)),
        ("insights admin denied analyze", ("insights", "Admin", no_analyze), {triage, events, runtime}),
        ("insights admin denied triage", ("insights", "Admin", no_triage), {analyze, events, runtime}),
        ("ALL admin denied analyze", ("ALL", "Admin", no_analyze), set(guards) - {analyze}),
    ]
    for name, (scope, level, rules), want in cases:
        async def resolve(_token, scope=scope, level=level, rules=rules):
            return "u", [{"status": "Active", "isExpired": False,
                          "scopes": [{"scope": scope, "level": level, "rules": rules}]}]

        monkeypatch.setattr(authz, "_resolve", resolve)
        allowed = set()
        for key, guard in guards.items():
            try:
                asyncio.run(guard(session_token="t"))
                allowed.add(key)
            except HTTPException as e:
                assert e.status_code == 403, name
        assert allowed == want, name


def test_denyable_rule_blocks_owner(monkeypatch):
    env = _env(monkeypatch)
    env.api.dependency_overrides.clear()
    rules = []

    async def owner(_token):
        return "u1", [{"status": "Active", "isExpired": False,
                       "scopes": [{"scope": "settings", "level": "Owner", "rules": rules}]}]

    monkeypatch.setattr(authz, "_resolve", owner)
    headers = {"X-Session-Token": "t"}
    assert env.client.post(VALIDATE, json={"model": "qwen3:4b"}, headers=headers).status_code == 200

    rules.append("settings.controlainsights.deny")
    r = env.client.post(VALIDATE, json={"model": "qwen3:4b"}, headers=headers)
    assert r.status_code == 403
    assert r.json() == {"status": 403, "operation": "Error", "message": "you do not have permission to perform this action"}
    assert env.client.post(PULL, json={"model": "qwen3:4b"}, headers=headers).status_code == 403


def test_errors_use_the_envelope(monkeypatch):
    env = _env(monkeypatch)
    r = env.client.post(VALIDATE, json={})
    assert r.status_code == 400 and r.json() == {"status": 400, "operation": "Error", "data": {"code": "invalid_request"}}

    env.api.dependency_overrides.clear()
    r = env.client.post(ANALYZE)
    assert r.status_code == 401
    assert r.json() == {"status": 401, "operation": "Error", "message": "a session token is required"}


# ---- analyze -----------------------------------------------------------------------
def test_analyze_disabled_409(monkeypatch):
    env = _env(monkeypatch, enabled=False)
    r = env.client.post(ANALYZE)
    assert r.status_code == 409
    assert r.json() == {"status": 409, "operation": "Error", "data": {"code": "analyzer_disabled"}}
    assert _xlen(env) == 0


def test_analyze_excluded_namespace_404(monkeypatch):
    env = _env(monkeypatch, excluded=[NS])
    r = env.client.post(ANALYZE)
    assert r.status_code == 404 and _code(r) == "app_not_found"
    assert env.exporter_calls == [] and _xlen(env) == 0


@pytest.mark.parametrize("path, app, state, status, code", [
    ("/api/v1/insights/applications/Shop/api/analyze", None, "ready", 400, "invalid_app"),
    ("/api/v1/insights/applications/shop/api_v2/analyze", None, "ready", 400, "invalid_app"),
    (ANALYZE, AppNotFound(NAME), "ready", 404, "app_not_found"),
    (ANALYZE, {"namespaces": {"items": [{"name": "other"}]}}, "ready", 404, "app_not_found"),
    (ANALYZE, {}, "ready", 404, "app_not_found"),
    (ANALYZE, None, "absent", 503, "runtime_absent"),
    (ANALYZE, None, "pulling", 503, "runtime_pulling"),
    (ANALYZE, ExporterUnavailable(500), "ready", 503, "storage_unavailable"),
])
def test_analyze_rejects(monkeypatch, path, app, state, status, code):
    env = _env(monkeypatch, app=app, state=state)
    r = env.client.post(path)
    assert r.status_code == status and _code(r) == code
    assert _xlen(env) == 0 and _doc(env).version == 0


@pytest.mark.parametrize("state", ["absent", "unreachable", "model_missing", "pulling", "unsupported"])
def test_analyze_fast_mode_accepts_unready_runtime(monkeypatch, state):
    # Rules need no model: the run writes rule cards and narrates only once the runtime is ready.
    env = _env(monkeypatch, state=state, mode="fast")
    r = env.client.post(ANALYZE)
    assert r.status_code == 202 and r.json()["data"]["status"] == "queued"
    assert _xlen(env) == 1


def test_analyze_deep_mode_503_when_unready(monkeypatch):
    env = _env(monkeypatch, state="model_missing", mode="deep")
    r = env.client.post(ANALYZE)
    assert r.status_code == 503 and _code(r) == "runtime_model_missing"
    assert _xlen(env) == 0


def test_analyze_queued_202(monkeypatch):
    env = _env(monkeypatch)
    sub = env.state.broadcaster.subscribe({"shop/api"})
    other = env.state.broadcaster.subscribe({"shop/web"})

    r = env.client.post(ANALYZE)

    assert r.status_code == 202
    run_id = r.json()["data"]["runId"]
    assert r.json() == {"status": 202, "operation": "Success", "data": {"runId": run_id, "status": "queued"}}
    assert _xlen(env) == 1
    assert env.fake.streams[STREAM] == [
        (run_id, {"namespace": NS, "name": NAME, "trigger": "manual", "generation": "7"})]
    doc = _doc(env)
    assert _drain(sub) == [
        ("analysis.queued", {"runId": run_id, "trigger": "manual", "version": doc.version, "app": "shop/api"})]
    assert _drain(other) == []


def test_analyze_writes_queued_last_run(monkeypatch):
    env = _env(monkeypatch)
    card = Insight(id="c1", subject="deployment/api", status="open")
    asyncio.run(env.state.store.put(NS, NAME, AppInsights(insights=[card], version=3)))

    run_id = env.client.post(ANALYZE).json()["data"]["runId"]

    doc = _doc(env)
    assert doc.version == 4 and doc.insights == [card]
    last = doc.lastRun
    assert (last.status, last.trigger, last.runId) == ("queued", "manual", run_id)
    assert last.queuedAt and not last.startedAt


def test_analyze_queued_even_when_the_stamp_fails(monkeypatch):
    env = _env(monkeypatch)
    sub = env.state.broadcaster.subscribe({"shop/api"})

    async def broken(*_args):
        raise RedisConnectionError("down")

    monkeypatch.setattr(env.state.store, "update", broken)
    r = env.client.post(ANALYZE)
    assert r.status_code == 202 and r.json()["data"]["status"] == "queued"
    assert _xlen(env) == 1 and _drain(sub) == []


def test_analyze_cooldown_429(monkeypatch):
    env = _env(monkeypatch)
    assert env.client.post(ANALYZE).status_code == 202
    r = env.client.post(ANALYZE)
    assert r.status_code == 429 and _code(r) == "cooldown_active"
    assert _xlen(env) == 1


def test_analyze_queue_full_429(monkeypatch):
    env = _env(monkeypatch)
    for _ in range(100):
        asyncio.run(env.fake.xadd(STREAM, {"namespace": "x", "name": "y", "trigger": "incident", "generation": "1"}))
    r = env.client.post(ANALYZE)
    assert r.status_code == 429 and _code(r) == "queue_full"
    assert _xlen(env) == 100 and _doc(env).version == 0


def test_queue_full_counts_backlog_only(monkeypatch):
    env = _env(monkeypatch)
    store = env.state.store

    async def fill_and_ack():
        await env.fake.xgroup_create(STREAM, "analyzer", id="0", mkstream=True)
        for _ in range(100):
            await env.fake.xadd(STREAM, {"namespace": "x", "name": "y", "trigger": "incident", "generation": "1"})
        return [msg_id for _s, entries in await env.fake.xreadgroup("analyzer", "w", {STREAM: ">"}, count=100)
                for msg_id, _f in entries]

    delivered = asyncio.run(fill_and_ack())
    # Delivered but unacknowledged messages are backlog too.
    assert env.client.post(ANALYZE).status_code == 429

    async def ack_all():
        for msg_id in delivered:
            await store.ack_job(msg_id)

    asyncio.run(ack_all())
    assert _xlen(env) == 0
    assert env.client.post(ANALYZE).status_code == 202


def test_analyze_running_202_while_inflight(monkeypatch):
    env = _env(monkeypatch)
    env.fake.store["analyzer:inflight:shop:api"] = "5-1"
    r = env.client.post(ANALYZE)
    assert r.status_code == 202 and r.json()["data"] == {"runId": "5-1", "status": "running"}
    assert _xlen(env) == 0 and "analyzer:cooldown:manual:shop:api" not in env.fake.store


def test_analyze_queues_while_a_sweep_review_holds_the_app(monkeypatch):
    env = _env(monkeypatch)
    env.fake.store["analyzer:inflight:shop:api"] = "review-1790000000000"
    r = env.client.post(ANALYZE)
    # Not the review's id: a real run is queued, and the worker waits for the review to end.
    assert r.status_code == 202 and r.json()["data"]["status"] == "queued"
    assert r.json()["data"]["runId"] != "review-1790000000000" and _xlen(env) == 1


def test_analyze_storage_unavailable_503(monkeypatch):
    env = _env(monkeypatch)

    async def down(*_args):
        raise RedisError("down")

    monkeypatch.setattr(env.fake, "xlen", down)
    r = env.client.post(ANALYZE)
    assert r.status_code == 503 and _code(r) == "storage_unavailable"


# ---- runtime, validate, pull ---------------------------------------------------------
def test_runtime_get(monkeypatch):
    env = _env(monkeypatch)
    r = env.client.get(RUNTIME)
    assert r.status_code == 200
    assert r.json() == {"status": 200, "operation": "Success",
                        "data": {"state": "ready", "model": "qwen3:4b", "reason": "", "mode": "deep",
                                 "autoPull": True, "enabled": False}}
    # ai.enabled reaches insights readers who cannot read the settings.
    env.api.state.runtime.set_enabled(True)
    assert env.client.get(RUNTIME).json()["data"]["enabled"] is True


def test_validate_codes(monkeypatch):
    env = _env(monkeypatch)

    r = env.client.post(VALIDATE, json={"model": "Bad Name"})
    assert r.status_code == 400 and _code(r) == "invalid_model_name"

    r = env.client.post(VALIDATE, json={"model": "qwen2.5:3b"})
    assert r.status_code == 404 and _code(r) == "model_not_installed"
    assert r.json()["data"]["license"] == "Qwen Research (non-commercial)" and r.json()["data"]["warning"]

    r = env.client.post(VALIDATE, json={"model": "qwen3:4b"})
    assert r.status_code == 200
    assert r.json()["data"] == {"ok": True, "model": "qwen3:4b", "license": "Apache-2.0", "warning": "",
                                "reason": "", "capabilities": ["completion", "tools"]}

    env.ollama.capabilities = ["completion"]
    r = env.client.post(VALIDATE, json={"model": "qwen3:4b"})
    assert r.status_code == 422 and _code(r) == "model_lacks_tools"
    assert r.json()["data"]["capabilities"] == ["completion"]

    env.ollama.overrides["/api/tags"] = httpx.Response(500, json={"error": "boom"})
    r = env.client.post(VALIDATE, json={"model": "qwen3:4b"})
    assert r.status_code == 503 and _code(r) == "runtime_unreachable"

    env.ollama.overrides["/api/tags"] = dns_error()
    r = env.client.post(VALIDATE, json={"model": "qwen3:4b"})
    assert r.status_code == 503 and _code(r) == "runtime_absent"
    # validate never changes the runtime state.
    assert env.state.runtime.status.state == "ready"


def test_pull(monkeypatch):
    env = _env(monkeypatch)
    runtime = env.state.runtime
    started = []

    def start_pull(model):
        started.append(model)
        runtime.status.state, runtime.status.model = "pulling", model

    monkeypatch.setattr(runtime, "start_pull", start_pull)

    r = env.client.post(PULL, json={"model": "Bad Name"})
    assert r.status_code == 400 and _code(r) == "invalid_model_name"

    monkeypatch.setattr(A, "OLLAMA_AUTO_PULL", False)
    r = env.client.post(PULL, json={"model": "qwen2.5:7b"})
    assert r.status_code == 409 and _code(r) == "auto_pull_disabled"
    assert started == []

    monkeypatch.setattr(A, "OLLAMA_AUTO_PULL", True)
    r = env.client.post(PULL, json={"model": "qwen2.5:7b"})
    assert r.status_code == 202
    assert r.json()["data"] == {"state": "pulling", "model": "qwen2.5:7b", "reason": "", "mode": "deep",
                                "autoPull": True, "enabled": False}
    assert started == ["qwen2.5:7b"]


def test_pull_while_pulling_returns_the_running_pull(monkeypatch):
    env = _env(monkeypatch, state="pulling")

    class Running:
        def done(self):
            return False

    env.state.runtime.pull_task = Running()
    r = env.client.post(PULL, json={"model": "qwen2.5:7b"})
    assert r.status_code == 202 and r.json()["data"]["model"] == "qwen3:4b"


# ---- events ----------------------------------------------------------------------------
def test_events_route_is_an_sse_stream(monkeypatch):
    env = _env(monkeypatch, excluded=["b"])
    resp = asyncio.run(_endpoint(env.api, EVENTS)(apps="a/x, b/y"))
    assert isinstance(resp, StreamingResponse)
    assert resp.media_type == "text/event-stream"
    assert resp.headers["x-accel-buffering"] == "no" and resp.headers["cache-control"] == "no-cache"
    (sub,) = env.state.broadcaster._subs
    assert sub.apps == {"a/x"}


def test_parse_apps_like_discovery():
    assert A.parse_apps("", []) == set()
    assert A.parse_apps(" a/x , ,b/y,noslash,c/d/e ", []) == {"a/x", "b/y", "c/d/e"}
    assert A.parse_apps("a/x,b/y", ["b"]) == {"a/x"}


def test_events_filters_excluded_apps():
    async def scenario():
        b = Broadcaster()
        sub = b.subscribe(A.parse_apps("a/x,b/y", ["b"]))
        stream = A.event_stream(b, sub, ping_s=0.01)
        assert await anext(stream) == ": connected\n\n"

        b.publish("insight.created", "b/y", {"id": "1", "version": 2, "status": "open"})
        b.publish("insight.created", "a/x", {"id": "2", "version": 3, "status": "open"})
        b.publish("runtime.changed", "", {"state": "ready", "model": "m", "reason": ""})

        assert await anext(stream) == (
            'event: insight.created\ndata: {"id": "2", "version": 3, "status": "open", "app": "a/x"}\n\n')
        assert await anext(stream) == (
            'event: runtime.changed\ndata: {"state": "ready", "model": "m", "reason": ""}\n\n')
        assert await anext(stream) == ": ping\n\n"
        # A ping does not end the stream: the next event still arrives.
        b.publish("resync", "a/x", {})
        assert await anext(stream) == 'event: resync\ndata: {"app": "a/x"}\n\n'

        await stream.aclose()
        b.publish("runtime.changed", "", {})
        assert sub.queue.empty()

    asyncio.run(scenario())


# ---- triage (S11) ------------------------------------------------------------------------------------------------
TRIAGE = "/api/v1/insights/applications/shop/api/insights/{}/triage"
TRIAGE_KEY = ("POST", "/api/v1/insights/applications/{namespace}/{name}/insights/{id}/triage")


def _triage_env(monkeypatch, user="u1", **kw):
    env = _env(monkeypatch, **kw)
    env.api.dependency_overrides[env.api.state.guards[TRIAGE_KEY]] = lambda: user
    rec = Insight(id="r1", kind="reliability", subject="deployment/api", status="open", category="recommendation",
                  reason="reliability.no_pdb", params={"workload": "api"})
    incident = Insight(id="i1", kind="oom", subject="deployment/api", status="open", category="incident")
    resolved = Insight(id="i2", kind="oom", subject="deployment/web", status="resolved", category="incident")
    asyncio.run(env.state.store.put(NS, NAME, AppInsights(insights=[rec, incident, resolved], version=5)))
    env.sub = env.state.broadcaster.subscribe({"shop/api"})
    return env


def test_triage_ok_publishes_updated(monkeypatch):
    env = _triage_env(monkeypatch)
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"})
    assert r.status_code == 200
    card = r.json()["data"]
    assert card["id"] == "r1" and card["triage"]["state"] == "dismissed" and card["triage"]["by"] == "u1"
    doc = _doc(env)
    assert doc.version == 6 and doc.insights[0].triage.state == "dismissed"
    assert _drain(env.sub) == [("insight.updated", {"id": "r1", "version": 6, "status": "open", "app": "shop/api"})]
    # The same action again changes nothing: 200, no write, no event.
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"})
    assert r.status_code == 200 and _doc(env).version == 6 and _drain(env.sub) == []
    # Acknowledge an incident, then reopen it.
    assert env.client.post(TRIAGE.format("i1"), json={"action": "acknowledge"}).json()["data"]["triage"]["state"] == (
        "acknowledged")
    assert "triage" not in env.client.post(TRIAGE.format("i1"), json={"action": "reopen"}).json()["data"]
    # Works while the analyzer is disabled: it only writes the card.
    env = _triage_env(monkeypatch, enabled=False)
    assert env.client.post(TRIAGE.format("r1"), json={"action": "acknowledge"}).status_code == 200


def test_triage_readonly_403(monkeypatch):
    env = _triage_env(monkeypatch)
    env.api.dependency_overrides.clear()

    async def readonly(_token):
        return "u2", [{"status": "Active", "isExpired": False,
                       "scopes": [{"scope": "insights", "level": "ReadOnly"}]}]

    monkeypatch.setattr(authz, "_resolve", readonly)
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"}, headers={"X-Session-Token": "t"})
    assert r.status_code == 403 and _doc(env).insights[0].triage is None

    async def contributor(_token):
        return "u3", [{"status": "Active", "isExpired": False,
                       "scopes": [{"scope": "insights", "level": "Contributor"}]}]

    monkeypatch.setattr(authz, "_resolve", contributor)
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"}, headers={"X-Session-Token": "t"})
    assert r.status_code == 200 and r.json()["data"]["triage"]["by"] == "u3", "the userID comes from auth-service"


def test_triage_unknown_id_404(monkeypatch):
    env = _triage_env(monkeypatch)
    r = env.client.post(TRIAGE.format("nope"), json={"action": "acknowledge"})
    assert (r.status_code, _code(r)) == (404, "insight_not_found")
    # An app never analyzed has no card at all.
    env.fake.store.clear()
    r = env.client.post(TRIAGE.format("r1"), json={"action": "acknowledge"})
    assert (r.status_code, _code(r)) == (404, "insight_not_found")


def test_triage_invalid_state_409(monkeypatch):
    env = _triage_env(monkeypatch)
    r = env.client.post(TRIAGE.format("i1"), json={"action": "dismiss"})
    assert (r.status_code, _code(r)) == (409, "invalid_triage")
    r = env.client.post(TRIAGE.format("i2"), json={"action": "acknowledge"})
    assert (r.status_code, _code(r)) == (409, "invalid_triage")
    assert _doc(env).version == 5


def test_triage_bad_body_400(monkeypatch):
    env = _triage_env(monkeypatch)
    for body in ({"action": "snooze"}, {}, None):
        r = env.client.post(TRIAGE.format("r1"), json=body)
        assert (r.status_code, _code(r)) == (400, "invalid_request")
    r = env.client.post("/api/v1/insights/applications/Shop/api/insights/r1/triage", json={"action": "reopen"})
    assert (r.status_code, _code(r)) == (400, "invalid_app")


def test_triage_excluded_namespace_404(monkeypatch):
    env = _triage_env(monkeypatch, excluded=["shop"])
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"})
    assert (r.status_code, _code(r)) == (404, "app_not_found") and env.exporter_calls == []
    for app in (AppNotFound("api"), {**APP, "namespaces": {"items": [{"name": "other"}]}}):
        env = _triage_env(monkeypatch, app=app)
        r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"})
        assert (r.status_code, _code(r)) == (404, "app_not_found")


def test_triage_storage_unavailable_503(monkeypatch):
    env = _triage_env(monkeypatch, app=ExporterUnavailable(503))
    r = env.client.post(TRIAGE.format("r1"), json={"action": "dismiss"})
    assert (r.status_code, _code(r)) == (503, "storage_unavailable")
