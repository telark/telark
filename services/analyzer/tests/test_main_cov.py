"""Checks for main.py: the worker task (stream consumer, drops, deep and fast runs, recovery
path), the config poll, the lifespan wiring and the entry point.

Redis is tests/fakes.FakeRedis whose stream raises Drained once nothing is left to
read (that ends worker_loop), Ollama is tests/fakes.FakeOllama (its /api/chat is the
fast narration), and exporter.current, exporter.get_application, analyzer.run_analysis
and analyzer.gather_rules are monkeypatched. Sleeps and the clock are injected, so no
test waits or reaches a model.

Run: python -m pytest tests/test_main_cov.py
"""

import asyncio
import hashlib
import os
import sys
from types import SimpleNamespace

import certifi
import httpx
import pytest
from redis import ConnectionError as RedisConnectionError

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import analyzer  # noqa: E402
import constants  # noqa: E402
import exporter  # noqa: E402
import main  # noqa: E402
import review  # noqa: E402

REVIEW_AFTER_RUN = main._review_after_run
from config import ANALYZER_CONFIG_POLL_SEC  # noqa: E402
from constants import CONSUMER_GROUP, JOB_MAX_AGE_S, STREAM_JOBS  # noqa: E402
from fakes import FakeOllama, FakeRedis  # noqa: E402
from insights import insight_id  # noqa: E402
from models import (  # noqa: E402
    AnalyzerConfig,
    AppInsights,
    Candidate,
    Emitted,
    EvidenceRef,
    Insight,
    LastRun,
    Outcome,
)
from tools.k8s_tools import K8s  # noqa: E402

ON = AnalyzerConfig(enabled=True, model="qwen3:4b", autoAnalyze=True)
FRESH_MS = 1000
STALE_MS = JOB_MAX_AGE_S * 1000 + 1
KEY = "analyzer:shop:api"
RECENT = "2026-09-23T10:00:00Z"


class Drained(Exception):
    """Ends worker_loop / config_poll: the stream has nothing left, or a sleep limit was hit."""


class Stream(FakeRedis):
    def __init__(self):
        super().__init__()
        self.reads = 0

    async def xreadgroup(self, *args, **kwargs):
        self.reads += 1
        out = await super().xreadgroup(*args, **kwargs)
        if not out:
            raise Drained
        return out


def _app(name, namespace="shop", workloads=None):
    return {
        "name": name,
        "namespaces": {"items": [{"name": namespace}]},
        "resources": [{"namespace": namespace, "kind": "Deployment", "name": w} for w in (workloads or [name])],
        "history": {"generation": 3, "changeLog": []},
    }


def _card(iid, subject, status="open", resolved_at=""):
    return Insight(id=iid, kind="crashloop", subject=subject, title="t", summary="s", confidence="low",
                   severity="warning", status=status, firstSeenAt=RECENT, lastSeenAt=RECENT,
                   resolvedAt=resolved_at, runs=1)


class Env:
    """One worker's dependencies (built once, reused across loops) plus what the test observes."""

    def __init__(self, monkeypatch, cfg=ON, ollama=None, apps=None, redis=None, mode="deep"):
        # First, so a missing worker is the red reason.
        self.worker = main.worker_loop
        self.cfg = cfg
        self.redis = redis or Stream()
        self.ollama = ollama or FakeOllama()
        self.apps = {"api": _app("api"), "web": _app("web")} if apps is None else apps
        self.analysis = lambda job, run: Outcome()
        self.candidates = lambda run: []
        self.runs = []
        self.sleeps = []
        events = main.Broadcaster()
        client = self.ollama.client()
        self.state = SimpleNamespace(
            redis=self.redis, store=main.InsightStore(self.redis), broadcaster=events,
            runtime=main.Runtime(client, events, mode=mode), ollama_client=client, exporter_client=None, k8s=None,
        )
        self.sub = events.subscribe({"shop/api", "shop/web", "shop/db", "hidden/svc"})
        monkeypatch.setattr(exporter, "current", lambda: self.cfg)
        monkeypatch.setattr(exporter, "get_application", self._get_application)
        monkeypatch.setattr(analyzer, "run_analysis", self._run_analysis)
        monkeypatch.setattr(analyzer, "gather_rules", self._gather_rules)
        # The setup review has its own tests below; here it only records that it was asked for.
        self.reviews = []

        async def review_after_run(state, job, app, run, run_id):
            self.reviews.append((job.name, run_id))

        monkeypatch.setattr(main, "_review_after_run", review_after_run)

    async def _get_application(self, client, name):
        app = self.apps.get(name)
        if isinstance(app, Exception):
            raise app
        if app is None:
            raise exporter.AppNotFound(name)
        return app

    async def _run_analysis(self, job, run, ollama_client, k8s, model):
        self.runs.append((job.name, model))
        return self.analysis(job, run)

    async def _gather_rules(self, run, k8s, now, timings):
        self.runs.append((run.name, "rules"))
        timings.update(gather=0.5, rules=0.001)
        return self.candidates(run)

    def add(self, trigger="incident", name="api", namespace="shop"):
        fields = {"namespace": namespace, "name": name, "trigger": trigger, "generation": "3"}
        return asyncio.run(self.redis.xadd(STREAM_JOBS, fields))

    def put(self, doc, key=KEY):
        self.redis.store[key] = doc.model_dump_json()

    def doc(self, key=KEY):
        raw = self.redis.store.get(key)
        return AppInsights.model_validate_json(raw) if raw else None

    def consume(self, now_ms=FRESH_MS, max_sleeps=0):
        """Runs worker_loop until the stream is drained or a sleep beyond max_sleeps."""

        async def sleep(seconds):
            self.sleeps.append(seconds)
            if len(self.sleeps) > max_sleeps:
                raise Drained

        async def go():
            with pytest.raises(Drained):
                await self.worker(self.state, sleep=sleep, clock_ms=lambda: now_ms, consumer="w")
            if self.state.runtime.pull_task:
                await self.state.runtime.pull_task

        asyncio.run(go())

    def poll(self, iterations=1):
        async def sleep(seconds):
            iterations_left[0] -= 1
            if not iterations_left[0]:
                raise Drained

        iterations_left = [iterations]

        async def go():
            with pytest.raises(Drained):
                await main.config_poll(self.state, sleep=sleep)
            if self.state.runtime.pull_task:
                await self.state.runtime.pull_task

        asyncio.run(go())

    def events(self):
        """The app-scoped events published so far (runtime.* are global and checked in test_runtime_cov)."""
        out = []
        while not self.sub.queue.empty():
            name, data = self.sub.queue.get_nowait()
            if "app" in data:
                out.append((name, data))
        return out

    def backlog(self):
        return asyncio.run(self.redis.xlen(STREAM_JOBS)), self.redis.groups[(STREAM_JOBS, CONSUMER_GROUP)]["pending"]


def test_disabled_does_not_consume(monkeypatch):
    env = Env(monkeypatch, cfg=AnalyzerConfig(enabled=False))
    env.add()
    env.consume()
    assert env.redis.reads == 0
    assert env.sleeps == [ANALYZER_CONFIG_POLL_SEC]
    assert env.backlog() == (1, {})


def test_auto_analyze_off_drops_incident(monkeypatch):
    env = Env(monkeypatch, cfg=ON.model_copy(update={"autoAnalyze": False}))
    env.add("incident")
    env.add("recovery")
    env.consume()
    assert env.runs == [] and env.ollama.paths == []
    assert env.backlog() == (0, {})
    assert env.events() == [] and KEY not in env.redis.store
    assert "analyzer:cooldown:auto:shop:api" not in env.redis.store


def test_recovery_resolves_without_model(monkeypatch):
    env = Env(monkeypatch)
    env.put(AppInsights(
        insights=[_card("a", "deployment/api"), _card("b", "deployment/web", status="updated"),
                  _card("c", "deployment/old", status="resolved", resolved_at="2020-01-01T00:00:00Z")],
        lastRun=LastRun(status="done", trigger="incident", runId="0-0"),
        version=4,
    ))
    msg = env.add("recovery")
    env.consume()

    assert env.ollama.paths == [] and env.runs == []
    doc = env.doc()
    assert [(c.id, c.status) for c in doc.insights] == [("a", "resolved"), ("b", "resolved")]
    assert (doc.lastRun.status, doc.lastRun.trigger, doc.lastRun.runId, doc.lastRun.error) == (
        "done", "recovery", msg, "")
    assert doc.lastRun.finishedAt and doc.version == 6
    events = env.events()
    assert [n for n, _ in events] == ["analysis.started", "insight.resolved", "insight.resolved", "analysis.finished"]
    assert events[0][1] == {"version": 5, "runId": msg, "model": "qwen3:4b", "app": "shop/api"}
    assert events[1][1] == {"id": "a", "version": 6, "status": "resolved", "app": "shop/api"}
    assert events[-1][1] == {"version": 6, "runId": msg, "truncated": False, "created": 0, "updated": 0,
                             "resolved": 2, "app": "shop/api"}
    assert env.backlog() == (0, {})


def test_stale_job_dropped(monkeypatch):
    env = Env(monkeypatch)
    env.add("incident")
    env.consume(now_ms=STALE_MS)
    assert env.backlog() == (0, {})
    assert env.runs == [] and env.events() == [] and env.ollama.paths == []
    assert "analyzer:cooldown:auto:shop:api" not in env.redis.store


def test_auto_cooldown(monkeypatch):
    env = Env(monkeypatch)
    first = env.add("incident")
    env.add("incident")
    env.consume()
    assert env.runs == [("api", "qwen3:4b")]
    doc = env.doc()
    assert (doc.lastRun.status, doc.lastRun.runId) == ("done", first)
    assert [n for n, _ in env.events()].count("analysis.started") == 1
    assert env.backlog() == (0, {})


def test_no_pull_when_disabled(monkeypatch):
    monkeypatch.setattr("runtime.OLLAMA_AUTO_PULL", True)
    monkeypatch.setattr(exporter, "refresh", lambda client: asyncio.sleep(0))
    ollama = FakeOllama(models=())
    env = Env(monkeypatch, cfg=AnalyzerConfig(enabled=False), ollama=ollama)
    env.poll()
    msg = env.add("incident")
    env.consume()
    assert env.state.runtime.status.state == "model_missing"
    assert ollama.count("/api/pull") == 0 and env.redis.reads == 0

    env.cfg = ON
    env.sleeps.clear()
    env.consume()

    assert ollama.count("/api/pull") == 1
    doc = env.doc()
    assert (doc.lastRun.status, doc.lastRun.error, doc.lastRun.runId) == ("failed", "model_not_installed", msg)
    events = env.events()
    (started,) = [d for n, d in events if n == "analysis.started"]
    failed = [d for n, d in events if n == "analysis.failed"]
    assert failed == [{"version": started["version"] + 1, "runId": msg, "error": "model_not_installed",
                       "app": "shop/api"}]
    # The worker pauses while the pull runs, and never reaches /api/chat.
    assert env.sleeps == [ANALYZER_CONFIG_POLL_SEC] and env.runs == []
    assert env.state.runtime.status.state == "ready"


def test_job_with_invalid_app_is_acked_unread(monkeypatch):
    env = Env(monkeypatch)
    key = "analyzer:x:foo/get?"
    env.redis.store[key] = "{}"
    env.add("manual", name="foo/get?", namespace="x")
    env.add("manual", namespace="Shop")
    env.consume()
    assert env.backlog() == (0, {})
    assert env.runs == [] and env.events() == [] and key in env.redis.store


def test_acked_jobs_leave_stream(monkeypatch):
    env = Env(monkeypatch)
    env.add("manual")
    env.add("incident", name="web")
    env.add("recovery")
    env.add("bogus")
    asyncio.run(env.redis.xadd(STREAM_JOBS, {"namespace": "shop"}))
    env.consume()
    assert env.backlog() == (0, {})
    assert [name for name, _ in env.runs] == ["api", "web"]


def test_manual_drop_clears_queued(monkeypatch):
    env = Env(monkeypatch)
    msg = env.add("manual")
    env.put(AppInsights(lastRun=LastRun(status="queued", trigger="manual", runId=msg, queuedAt=RECENT), version=5))
    env.consume(now_ms=STALE_MS)

    last = env.doc().lastRun
    assert (last.status, last.error, last.runId, last.queuedAt, env.doc().version) == (
        "failed", "job_expired", msg, RECENT, 6)
    assert last.finishedAt
    assert env.events() == [("analysis.failed", {"version": 6, "runId": msg, "error": "job_expired", "app": "shop/api"})]
    assert env.backlog() == (0, {}) and env.runs == []


def test_manual_drop_keeps_other_run(monkeypatch):
    env = Env(monkeypatch)
    env.add("manual")
    other = AppInsights(lastRun=LastRun(status="running", trigger="incident", runId="9-9", startedAt=RECENT), version=5)
    env.put(other)
    env.consume(now_ms=STALE_MS)
    assert env.doc() == other
    assert env.events() == []
    assert env.backlog() == (0, {})


def test_reclaimed_running_cleared(monkeypatch):
    env = Env(monkeypatch)
    msg = env.add("manual")
    # A worker that crashed mid-run: the message is pending on its consumer, lastRun shows it running.
    asyncio.run(env.redis.xgroup_create(STREAM_JOBS, CONSUMER_GROUP, id="0"))
    asyncio.run(env.redis.xreadgroup(CONSUMER_GROUP, "crashed", {STREAM_JOBS: ">"}, count=1))
    env.put(AppInsights(lastRun=LastRun(status="running", trigger="manual", runId=msg, startedAt=RECENT), version=7))
    env.redis.clock_ms = STALE_MS
    env.consume(now_ms=STALE_MS)

    doc = env.doc()
    assert (doc.lastRun.status, doc.lastRun.error, doc.version) == ("failed", "job_expired", 8)
    assert env.events() == [("analysis.failed", {"version": 8, "runId": msg, "error": "job_expired", "app": "shop/api"})]
    assert env.backlog() == (0, {}) and env.runs == []


def test_dead_consumers_are_forgotten(monkeypatch):
    # Live: one consumer per past analyzer pod accumulated in the group.
    env = Env(monkeypatch)
    asyncio.run(env.redis.xgroup_create(STREAM_JOBS, CONSUMER_GROUP, id="0", mkstream=True))
    now = constants.CONSUMER_MAX_IDLE_MS + 10
    env.redis.clock_ms = now
    env.redis.consumers[(STREAM_JOBS, CONSUMER_GROUP)] = {"gone": 0, "recent": now - 1000, "holding": 0}
    # A message it still holds keeps a consumer, however idle: deleting it would drop the message.
    env.redis.groups[(STREAM_JOBS, CONSUMER_GROUP)]["pending"]["1-1"] = ("holding", now)
    env.consume(now_ms=now)
    assert set(env.redis.consumers[(STREAM_JOBS, CONSUMER_GROUP)]) == {"w", "recent", "holding"}


def _history_at(generation):
    app = _app("api")
    app["history"] = {"generation": generation, "changeLog": []}
    return app


@pytest.mark.parametrize(("trigger", "reads", "sleeps", "seen"), [
    # Live (L-8): discovery enqueued the incident before exporter held its entry.
    ("incident", [2, 3], 1, (3, 3)),
    # Never caught up: bounded, and the rules then cite no older change.
    ("incident", [2, 2, 2], 2, (2, 3)),
    ("manual", [2], 0, (2, 0)),
])
def test_incident_run_waits_for_its_change(monkeypatch, trigger, reads, sleeps, seen):
    env = Env(monkeypatch)
    apps = [_history_at(g) for g in reads]

    async def get_application(client, name):
        return apps.pop(0)

    monkeypatch.setattr(exporter, "get_application", get_application)
    runs = []
    env.analysis = lambda job, run: runs.append((run.app["history"]["generation"], run.min_generation)) or Outcome()
    env.add(trigger)
    env.consume(max_sleeps=sleeps)
    assert env.sleeps == [constants.APP_CATCHUP_INTERVAL_S] * sleeps and runs == [seen] and apps == []


def test_execute_exception_fails_run(monkeypatch):
    env = Env(monkeypatch)
    first = env.add("manual")
    env.add("manual", name="web")

    def analysis(job, run):
        if job.name == "api":
            raise RuntimeError("raw model output")
        return Outcome()

    env.analysis = analysis
    env.consume()

    assert (env.doc().lastRun.status, env.doc().lastRun.error) == ("failed", "internal_error")
    assert env.doc("analyzer:shop:web").lastRun.status == "done"
    events = env.events()
    (started, *_) = [d for n, d in events if n == "analysis.started"]
    failed = [d for n, d in events if n == "analysis.failed"]
    assert failed == [{"version": started["version"] + 1, "runId": first, "error": "internal_error", "app": "shop/api"}]
    assert "analyzer:inflight:shop:api" not in env.redis.store
    assert env.backlog() == (0, {})


def test_inflight_held_is_run_in_progress(monkeypatch):
    env = Env(monkeypatch)
    msg = env.add("manual")
    env.put(AppInsights(lastRun=LastRun(status="queued", trigger="manual", runId=msg, queuedAt=RECENT), version=1))
    env.redis.store["analyzer:inflight:shop:api"] = "0-99"
    env.consume()

    assert (env.doc().lastRun.status, env.doc().lastRun.error) == ("failed", "run_in_progress")
    assert env.events() == [("analysis.failed", {"version": 2, "runId": msg, "error": "run_in_progress",
                                                 "app": "shop/api"})]
    assert env.redis.store["analyzer:inflight:shop:api"] == "0-99"
    assert env.runs == [] and env.backlog() == (0, {})


def _consume_under_review(env, release_after=None):
    """Runs the worker while a sweep review holds the app; the review ends after `release_after` retry sleeps."""
    key = "analyzer:inflight:shop:api"
    env.redis.store[key] = "review-1790000000000"

    async def sleep(seconds):
        env.sleeps.append(seconds)
        if len(env.sleeps) == release_after:
            del env.redis.store[key]

    async def go():
        with pytest.raises(Drained):
            await env.worker(env.state, sleep=sleep, clock_ms=lambda: FRESH_MS, consumer="w")

    asyncio.run(go())


def test_job_waits_for_a_sweep_review_of_its_app(monkeypatch):
    env = Env(monkeypatch)
    msg = env.add("manual")
    _consume_under_review(env, release_after=2)
    assert env.sleeps == [constants.INFLIGHT_RETRY_S] * 2
    assert (env.doc().lastRun.status, env.doc().lastRun.runId) == ("done", msg) and env.runs == [("api", "qwen3:4b")]
    assert "analyzer:inflight:shop:api" not in env.redis.store and env.backlog() == (0, {})


def test_sweep_review_past_its_wall_is_run_in_progress(monkeypatch):
    env = Env(monkeypatch)
    msg = env.add("manual")
    env.put(AppInsights(lastRun=LastRun(status="queued", trigger="manual", runId=msg, queuedAt=RECENT), version=1))
    _consume_under_review(env)
    assert env.sleeps == [constants.INFLIGHT_RETRY_S] * int(constants.REVIEW_WALL_S / constants.INFLIGHT_RETRY_S)
    assert (env.doc().lastRun.status, env.doc().lastRun.error) == ("failed", "run_in_progress") and env.runs == []
    assert env.redis.store["analyzer:inflight:shop:api"] == "review-1790000000000"


def test_run_merges_and_announces(monkeypatch):
    env = Env(monkeypatch, apps={"api": _app("api", workloads=["api", "worker"])})
    api_id = insight_id("shop", "api", "deployment/api", "shop")
    worker_id = insight_id("shop", "api", "deployment/worker", "shop")
    msg = env.add("manual")
    env.put(AppInsights(insights=[_card(api_id, "deployment/api", status="resolved", resolved_at=RECENT)],
                        lastRun=LastRun(status="queued", trigger="manual", runId=msg, queuedAt=RECENT), version=2))

    def emitted(subject):
        return Emitted(kind="crashloop", subject=subject, title="crash", summary="exits", confidence="high",
                       severity="critical", evidence=[EvidenceRef(type="workload", ref="workload:" + subject),
                                                      EvidenceRef(type="change", ref="gen:3")])

    def analysis(job, run):
        run.refs.update({"workload:deployment/api", "workload:deployment/worker", "gen:3"})
        return Outcome(emitted=[emitted("deployment/api"), emitted("deployment/worker")], steps=3, tool_calls=2)

    env.analysis = analysis
    env.consume()

    doc = env.doc()
    assert {c.id: (c.status, c.runs) for c in doc.insights} == {api_id: ("open", 2), worker_id: ("open", 1)}
    last = doc.lastRun
    assert (last.status, last.trigger, last.runId, last.queuedAt, last.model) == ("done", "manual", msg, RECENT, "qwen3:4b")
    assert (last.steps, last.toolCalls, last.truncated, doc.version) == (3, 2, False, 4)
    events = env.events()
    assert events == [
        ("analysis.started", {"version": 3, "runId": msg, "model": "qwen3:4b", "app": "shop/api"}),
        ("insight.created", {"id": worker_id, "version": 4, "status": "open", "app": "shop/api"}),
        ("insight.updated", {"id": api_id, "version": 4, "status": "open", "app": "shop/api"}),
        ("analysis.finished", {"version": 4, "runId": msg, "truncated": False, "created": 1, "updated": 1,
                               "resolved": 0, "app": "shop/api"}),
    ]


def test_run_error_keeps_insights(monkeypatch):
    env = Env(monkeypatch)
    card = _card("a", "deployment/api")
    env.put(AppInsights(insights=[card], version=1))
    msg = env.add("manual")
    env.analysis = lambda job, run: Outcome(error="model_timeout", steps=2, tool_calls=1)
    env.consume()

    doc = env.doc()
    assert doc.insights == [card]
    assert (doc.lastRun.status, doc.lastRun.error, doc.lastRun.steps, doc.lastRun.toolCalls, doc.version) == (
        "failed", "model_timeout", 2, 1, 3)
    assert env.events()[-1] == ("analysis.failed", {"version": 3, "runId": msg, "error": "model_timeout",
                                                    "app": "shop/api"})


def test_app_gone_deletes_document(monkeypatch):
    apps = {"web": {"name": "web"}, "db": _app("db", namespace="other"), "svc": _app("svc", namespace="hidden")}
    env = Env(monkeypatch, apps=apps, cfg=ON.model_copy(update={"excludedNamespaces": ["hidden"]}))
    jobs = [("api", "shop"), ("web", "shop"), ("db", "shop"), ("svc", "hidden")]
    for name, namespace in jobs:
        env.put(AppInsights(version=3, lastRun=LastRun(status="done")), key=f"analyzer:{namespace}:{name}")
    ids = [env.add("manual", name=name, namespace=namespace) for name, namespace in jobs]
    env.consume()

    assert not [k for k in env.redis.store if k.startswith("analyzer:") and k.count(":") == 2]
    assert env.events() == [
        ("analysis.failed", {"version": 0, "runId": msg, "error": "app_not_found", "app": f"{ns}/{name}"})
        for msg, (name, ns) in zip(ids, jobs)
    ]
    assert env.runs == [] and env.backlog() == (0, {})


def test_exporter_down_is_storage_unavailable(monkeypatch):
    env = Env(monkeypatch, apps={"api": exporter.ExporterUnavailable(503)})
    msg = env.add("manual")
    env.consume()
    assert (env.doc().lastRun.status, env.doc().lastRun.error, env.doc().lastRun.runId) == (
        "failed", "storage_unavailable", msg)
    assert [n for n, _ in env.events()] == ["analysis.failed"]


def test_failure_write_is_best_effort(monkeypatch):
    class DocumentDown(Stream):
        async def set(self, key, value, nx=False, ex=None):
            if key == KEY:
                raise RedisConnectionError("down")
            return await super().set(key, value, nx=nx, ex=ex)

    env = Env(monkeypatch, redis=DocumentDown())
    env.add("manual")
    env.consume()
    assert env.events() == [] and KEY not in env.redis.store
    assert env.backlog() == (0, {})


@pytest.mark.parametrize("setup, code, backoff", [
    (lambda o: o.overrides.update({"/api/tags": httpx.ConnectError("refused")}), "runtime_unreachable", True),
    (lambda o: o.capabilities.remove("tools"), "model_unsupported", False),
])
def test_runtime_not_ready_fails_run(monkeypatch, setup, code, backoff):
    ollama = FakeOllama()
    setup(ollama)
    env = Env(monkeypatch, ollama=ollama)
    env.add("manual")
    env.consume(max_sleeps=1)
    assert (env.doc().lastRun.status, env.doc().lastRun.error) == ("failed", code)
    assert env.sleeps == ([constants.WORKER_BACKOFF_S] if backoff else [])
    assert env.runs == [] and ollama.count("/api/pull") == 0


def test_redis_errors_back_off(monkeypatch):
    class Flaky(Stream):
        failures = {"xgroup_create": 1, "xreadgroup": 1}

        def _fail(self, name):
            if self.failures[name]:
                self.failures[name] -= 1
                raise RedisConnectionError("down")

        async def xgroup_create(self, *args, **kwargs):
            self._fail("xgroup_create")
            return await super().xgroup_create(*args, **kwargs)

        async def xreadgroup(self, *args, **kwargs):
            self._fail("xreadgroup")
            return await super().xreadgroup(*args, **kwargs)

    env = Env(monkeypatch, redis=Flaky())
    env.add("manual")
    env.consume(max_sleeps=2)
    assert env.sleeps == [constants.WORKER_BACKOFF_S] * 2
    assert env.doc().lastRun.status == "done" and env.backlog() == (0, {})


def test_config_poll_survives_errors(monkeypatch):
    refreshed = []

    async def refresh(client):
        refreshed.append(client)

    monkeypatch.setattr(exporter, "refresh", refresh)
    env = Env(monkeypatch)

    async def broken(model):
        raise ValueError("bad body")

    env.state.runtime.check = broken
    env.poll(iterations=2)
    assert len(refreshed) == 2


@pytest.mark.parametrize(("auto_pull", "installed", "pulls", "state"), [
    (True, (), 1, "ready"),
    (False, (), 0, "model_missing"),
    (True, ("qwen3:4b",), 0, "ready"),
])
def test_config_poll_pulls_a_missing_model_once(monkeypatch, auto_pull, installed, pulls, state):
    monkeypatch.setattr("runtime.OLLAMA_AUTO_PULL", auto_pull)
    monkeypatch.setattr(exporter, "refresh", lambda client: asyncio.sleep(0))
    ollama = FakeOllama(models=installed)
    env = Env(monkeypatch, ollama=ollama)
    env.poll(iterations=3)
    assert ollama.count("/api/pull") == pulls
    assert env.state.runtime.status.state == state
    # ai.enabled rides on the runtime status for insights readers who cannot read the settings.
    assert env.state.runtime.status.enabled is True


@pytest.mark.parametrize("ca_path", [certifi.where(), "/nonexistent/ca.crt"])
def test_lifespan_wires_state_and_stops_tasks(monkeypatch, ca_path):
    started, closed = [], []

    async def forever(state):
        started.append(state)
        await asyncio.Event().wait()

    class Redis:
        async def aclose(self):
            closed.append("redis")

    monkeypatch.setattr(main, "config_poll", forever)
    monkeypatch.setattr(main, "worker_loop", forever)
    monkeypatch.setattr(main, "review_loop", forever)
    monkeypatch.setattr(main, "SA_CA_PATH", ca_path)
    app = main.create_app(Redis(), main.lifespan)

    async def go():
        async with main.lifespan(app):
            await asyncio.sleep(0)
            s = app.state
            assert started == [s, s, s] and not closed
            assert isinstance(s.store, main.InsightStore) and isinstance(s.runtime, main.Runtime)
            assert s.runtime.status.state == "unreachable"
            s.runtime.pull_task = asyncio.create_task(asyncio.Event().wait())
        return s

    s = asyncio.run(go())
    assert closed == ["redis"]
    assert s.ollama_client.is_closed and s.exporter_client.is_closed
    assert s.runtime.pull_task.cancelled()
    if ca_path == certifi.where():
        assert isinstance(s.k8s, K8s) and s.k8s._client.is_closed
    else:
        assert s.k8s is None


def test_serve_builds_app_and_graceful_config(monkeypatch):
    fake = FakeRedis()
    seen = {}

    def from_url(url, **kwargs):
        seen["redis"] = (url, kwargs)
        return fake

    class Server:
        def __init__(self, config):
            seen["config"] = config

        async def serve(self):
            seen["served"] = True

    monkeypatch.setattr(main.aioredis.Redis, "from_url", from_url)
    monkeypatch.setattr(main.uvicorn, "Server", Server)
    monkeypatch.setattr(main, "REDIS_PASSWORD", "test-password")
    asyncio.run(main.serve())

    config = seen["config"]
    assert seen["served"] is True
    assert config.timeout_graceful_shutdown == 5
    assert config.app.state.redis is fake
    assert config.app.router.lifespan_context is not None
    assert seen["redis"][1] == {
        "password": "test-password",
        "max_connections": main.REDIS_POOL_SIZE,
        "decode_responses": True,
    }


def test_run_configures_logging_then_serves(monkeypatch):
    calls = []

    async def serve():
        calls.append("serve")

    monkeypatch.setattr(main, "configure", lambda level: calls.append(level))
    monkeypatch.setattr(main, "serve", serve)
    main.run()
    assert calls == [main.LOG_LEVEL, "serve"]


# ---- fast mode: rules first, one narration, two writes -------------------------------------
class Recording(Stream):
    """Keeps every write of the api document, so a test sees the run's intermediate state."""

    def __init__(self):
        super().__init__()
        self.docs = []

    async def set(self, key, value, nx=False, ex=None):
        if key == KEY:
            self.docs.append(AppInsights.model_validate_json(value))
        return await super().set(key, value, nx=nx, ex=ex)


API_ID = insight_id("shop", "api", "deployment/api", "shop")
WEB_ID = insight_id("shop", "api", "deployment/web", "shop")
TEMPLATE = ("api is crash-looping", "Pod api-1: CrashLoopBackOff, 5 restarts.")
NARRATED = ("api keeps crashing", "The api pod restarted 5 times.")


def _crashloop(run, truncated=False):
    """What gather_rules leaves behind: the refs and caches of its reads, web seen healthy, one api card."""
    run.refs.update({"workload:deployment/api", "workload:deployment/web", "gen:3"})
    run.status_cache[("shop", "deployment/web")] = {"ready": 2, "desired": 2, "pods": []}
    run.tool_calls, run.truncated = 4, truncated
    return [Candidate(subject="deployment/api", kind="crashloop", severity="critical", confidence="high",
                      evidence=[EvidenceRef(type="workload", ref="workload:deployment/api"),
                                EvidenceRef(type="change", ref="gen:3")],
                      title=TEMPLATE[0], summary=TEMPLATE[1], facts=["restarts: 5"])]


def _fast(monkeypatch, ollama=None, truncated=False, narration=NARRATED):
    env = Env(monkeypatch, mode="fast", ollama=ollama, redis=Recording(),
              apps={"api": _app("api", workloads=["api", "web"])})
    env.put(AppInsights(insights=[_card(WEB_ID, "deployment/web")], version=1))
    env.candidates = lambda run: _crashloop(run, truncated)
    env.ollama.narration = [narration] if narration else []
    return env


def _api_card(doc):
    return next(c for c in doc.insights if c.id == API_ID)


def test_fast_run_writes_twice_and_finishes(monkeypatch):
    env = _fast(monkeypatch)
    msg = env.add("manual")
    env.consume()

    start, cards, done = env.redis.docs
    assert start.lastRun.status == "running" and start.insights == [_card(WEB_ID, "deployment/web")]
    # First write: the rule cards, visible while the run is still running.
    assert cards.lastRun.status == "running" and cards.version == 3
    assert (_api_card(cards).title, _api_card(cards).summary, _api_card(cards).status) == (*TEMPLATE, "open")
    # Second write: narrated prose, the observed recovery, the finished run.
    card = _api_card(done)
    assert (card.title, card.summary) == NARRATED
    assert (card.kind, card.severity, card.confidence, card.runs, card.status) == (
        "crashloop", "critical", "high", 1, "open")
    assert [e.ref for e in card.evidence] == ["workload:deployment/api", "gen:3"]
    assert card.lastSeenAt == _api_card(cards).lastSeenAt
    assert next(c for c in done.insights if c.id == WEB_ID).status == "resolved"
    last = done.lastRun
    assert (last.status, last.runId, last.steps, last.toolCalls, last.truncated, last.error, done.version) == (
        "done", msg, 1, 4, False, "", 4)
    assert env.ollama.count("/api/chat") == 1 and env.runs == [("api", "rules")]
    assert env.events() == [
        ("analysis.started", {"version": 2, "runId": msg, "model": "qwen3:4b", "app": "shop/api"}),
        ("insight.created", {"id": API_ID, "version": 3, "status": "open", "app": "shop/api"}),
        ("insight.updated", {"id": API_ID, "version": 4, "status": "open", "app": "shop/api"}),
        ("insight.resolved", {"id": WEB_ID, "version": 4, "status": "resolved", "app": "shop/api"}),
        ("analysis.finished", {"version": 4, "runId": msg, "truncated": False, "created": 1, "updated": 0,
                               "resolved": 1, "app": "shop/api"}),
    ]


def test_fast_run_not_ready_still_writes_cards(monkeypatch):
    monkeypatch.setattr("runtime.OLLAMA_AUTO_PULL", True)
    env = _fast(monkeypatch, ollama=FakeOllama(models=()))
    env.add("manual")
    env.consume()

    doc = env.doc()
    assert (doc.lastRun.status, doc.lastRun.error, doc.lastRun.steps) == ("done", "", 0)
    assert (_api_card(doc).title, _api_card(doc).summary) == TEMPLATE
    # The run asked for the model, so the next run narrates; this one never waited for it.
    assert env.ollama.count("/api/pull") == 1 and env.ollama.count("/api/chat") == 0
    assert env.state.runtime.status.state == "ready"
    assert [n for n, _ in env.events()] == ["analysis.started", "insight.created", "insight.resolved",
                                            "analysis.finished"]


def test_fast_run_narration_ollama_500_still_done(monkeypatch):
    env = _fast(monkeypatch, truncated=True)
    env.ollama.overrides["/api/chat"] = httpx.Response(500, json={"error": "model requires more system memory"})
    msg = env.add("manual")
    env.consume()

    doc = env.doc()
    assert (doc.lastRun.status, doc.lastRun.error, doc.lastRun.steps, doc.lastRun.truncated) == ("done", "", 0, True)
    card = _api_card(doc)
    # Templates kept; a truncated run is low confidence and resolves nothing.
    assert ((card.title, card.summary), card.confidence) == (TEMPLATE, "low")
    assert next(c for c in doc.insights if c.id == WEB_ID).status == "open"
    events = env.events()
    assert [n for n, _ in events] == ["analysis.started", "insight.created", "analysis.finished"]
    assert events[-1][1] == {"version": 4, "runId": msg, "truncated": True, "created": 1, "updated": 0,
                             "resolved": 0, "app": "shop/api"}


def test_upgrade_keeps_one_card_per_incident(monkeypatch):
    """A document written before incident ids named the workload namespace, then a run of the new code."""
    def legacy(subject):
        return hashlib.sha256(f"shop|api|{subject}".encode()).hexdigest()[:16]

    api = _card(legacy("deployment/api"), "deployment/api")
    api.params = {"workload": "api", "namespace": "shop"}  # a fast-mode card
    web = _card(legacy("deployment/web"), "deployment/web")  # a deep-mode card: no params
    env = _fast(monkeypatch)
    env.put(AppInsights(insights=[api, web], version=1))
    env.add("manual")
    env.consume()

    doc = env.doc()
    # The incident still firing keeps its card under the new id; the recovered one resolves, nothing is orphaned.
    assert [(c.id, c.status, c.runs, c.firstSeenAt) for c in doc.insights] == [
        (API_ID, "updated", 2, RECENT), (web.id, "resolved", 1, RECENT)]
    assert (_api_card(doc).title, _api_card(doc).params["namespace"]) == (NARRATED[0], "shop")


def test_fast_run_publishes_finished_once(monkeypatch):
    # Blank narration keeps the templates (nothing applied: steps 0); an existing card is an update.
    env = _fast(monkeypatch, narration=("  ", ""))
    env.put(AppInsights(insights=[_card(API_ID, "deployment/api")], version=1))
    env.add("manual")
    env.consume()

    names = [n for n, _ in env.events()]
    assert names.count("analysis.finished") == 1 and "analysis.failed" not in names
    assert names == ["analysis.started", "insight.updated", "analysis.finished"]
    doc = env.doc()
    assert (_api_card(doc).title, _api_card(doc).runs, doc.lastRun.steps) == (TEMPLATE[0], 2, 0)


def test_fast_run_no_candidates_skips_narration(monkeypatch):
    env = Env(monkeypatch, mode="fast")
    msg = env.add("manual")
    env.consume()
    assert env.ollama.count("/api/chat") == 0 and env.ollama.count("/api/show") == 1
    assert (env.doc().lastRun.status, env.doc().lastRun.steps, env.doc().insights) == ("done", 0, [])
    events = env.events()
    assert events[-1] == ("analysis.finished", {"version": events[0][1]["version"] + 2, "runId": msg,
                                                "truncated": False, "created": 0, "updated": 0, "resolved": 0,
                                                "app": "shop/api"})


def test_fast_worker_runs_while_pulling(monkeypatch):
    env = _fast(monkeypatch)
    monkeypatch.setattr(main.Runtime, "pulling", property(lambda self: True))
    env.add("manual")
    env.consume()
    # Deep mode idles while a model pulls; fast runs the rules and skips only the narration.
    assert env.sleeps == [] and env.runs == [("api", "rules")]
    assert (env.doc().lastRun.status, env.doc().lastRun.steps) == ("done", 0)
    assert env.ollama.paths == []


def test_run_logs_timings(monkeypatch):
    lines = []
    monkeypatch.setattr(main, "logger", SimpleNamespace(info=lambda msg, *args: lines.append(msg.format(*args))))
    monkeypatch.setattr("runtime.OLLAMA_AUTO_PULL", False)
    env = _fast(monkeypatch)
    msg = env.add("manual")
    env.consume()
    env.ollama.overrides["/api/show"] = httpx.Response(404, json={"error": "not found"})
    second = env.add("manual", name="web")
    env.apps["web"] = _app("web")
    env.consume()

    first_line, missing_line = lines
    assert first_line.startswith(f"run {msg} timings: gather=0.5s rules=0.001s narrate=")
    assert first_line.endswith("narrated=True reason=")
    assert missing_line.startswith(f"run {second} timings:") and missing_line.endswith("narrated=False reason=model_missing")
    # The runId only: the service never logs customer namespaces or names.
    assert not any(word in line for line in lines for word in ("shop", "api", "web"))


# ---- setup reviews: after every analysis run, and the sweep (S11) ---------------------------------------------
import specs  # noqa: E402

REC_REASON = "reliability.single_replica"


def _single_replica_inputs(name="api"):
    return specs.build([specs.workload(name, replicas=1)])


def _reviewing(env, monkeypatch, factory=_single_replica_inputs, fail=None):
    monkeypatch.setattr(main, "_review_after_run", REVIEW_AFTER_RUN)
    env.gathered = []

    async def gather(app, doc, run, k8s, client, redis, ns_cache=None, in_run=False):
        env.gathered.append((run.namespace, run.name, in_run, ns_cache))
        if fail:
            raise fail
        return factory(run.name)

    monkeypatch.setattr(review, "gather", gather)
    return env


def test_run_reviews_after_final_write(monkeypatch):
    env = _reviewing(_fast(monkeypatch), monkeypatch)
    msg = env.add("manual")
    env.consume()
    start, cards, done, reviewed = env.redis.docs
    assert done.lastRun.status == "done" and done.lastReviewAt == ""
    # The review's own write: recommendations, lastReviewAt and a new version; lastRun untouched.
    assert reviewed.lastRun == done.lastRun and reviewed.version == done.version + 1 and reviewed.lastReviewAt
    (rec,) = [c for c in reviewed.insights if c.category == "recommendation"]
    assert (rec.reason, rec.subject, rec.status) == (REC_REASON, "deployment/api", "open")
    assert env.gathered == [("shop", "api", True, None)]
    events = env.events()
    names = [name for name, _ in events]
    assert names[-3:] == ["analysis.finished", "insight.created", "review.finished"], (
        "analysis.finished is never delayed")
    # The review's write is announced even when no card changes: the open panel refetches on its version.
    assert events[-1][1] == {"version": reviewed.version, "app": "shop/api"}
    assert env.redis.zsets["analyzer:index"]["shop/api"] > 0
    assert env.redis.hashes["analyzer:review"]["shop/api"].startswith("3:")
    assert msg in env.redis.docs[-1].lastRun.runId


def test_review_failure_keeps_run_done(monkeypatch):
    env = _reviewing(_fast(monkeypatch), monkeypatch, fail=RuntimeError("boom"))
    warnings = []
    monkeypatch.setattr(main.logger, "warning", lambda *args: warnings.append(args))
    env.add("manual")
    env.consume()
    assert env.doc().lastRun.status == "done" and len(env.redis.docs) == 3
    assert ("review failed: {}", "RuntimeError") in warnings


def test_recovery_run_has_no_review(monkeypatch):
    env = Env(monkeypatch)
    env.add("recovery")
    env.consume()
    assert env.reviews == []
    env = Env(monkeypatch, mode="fast")
    msg = env.add("manual")
    env.consume()
    assert env.reviews == [("api", msg)]


def test_deep_run_reviews_after_announce(monkeypatch):
    env = Env(monkeypatch)
    seen = []

    async def review_after_run(state, job, app, run, run_id):
        seen.append([name for name, _ in env.events()])

    monkeypatch.setattr(main, "_review_after_run", review_after_run)
    env.add("manual")
    env.consume()
    assert seen and seen[0][-1] == "analysis.finished"


def test_run_skips_review_when_queue_not_empty(monkeypatch):
    env = _reviewing(_fast(monkeypatch), monkeypatch)
    env.add("manual")
    env.add("manual", name="web")
    job = main.Job(namespace="shop", name="api", trigger="manual", generation=3)
    asyncio.run(main._review_after_run(env.state, job, _app("api"), main.Run("shop", "api", _app("api"), []), "1-1"))
    assert env.gathered == []


def _sweep(env, monkeypatch, apps, clock=1_800_000_000.0, missing=None, fail=None):
    listed = []

    async def list_applications(client):
        listed.append(True)
        if fail:
            raise fail
        return apps

    monkeypatch.setattr(exporter, "list_applications", list_applications)
    missing = {} if missing is None else missing
    asyncio.run(main.review_tick(env.state, missing, clock=lambda: clock))
    return listed, missing


def _summary(name, generation=3, namespace="shop"):
    return {"name": name, "namespaces": {"items": [{"name": namespace}]}, "history": {"generation": generation}}


def test_sweep_orders_generation_changes_first(monkeypatch):
    now = 1_800_000_000
    apps = {n: _app(n) for n in ("a", "b", "c", "d")}
    env = _reviewing(Env(monkeypatch, apps=apps), monkeypatch)
    env.redis.hashes["analyzer:review"] = {
        "shop/a": f"3:{now - 60}",            # fresh, same generation: not due
        "shop/b": f"3:{now - 8000}",          # older than the interval: due
        "shop/c": f"2:{now - 60}",            # generation changed: due first
        "shop/d": "3:bogus",                  # unreadable record: due (never reviewed)
    }
    _sweep(env, monkeypatch, [_summary(n) for n in ("a", "b", "c", "d")] + [_summary("x", namespace="hidden")],
           clock=now)
    assert [name for _, name, _, _ in env.gathered] == ["c", "d", "b"]
    assert all(not in_run and cache is not None for _, _, in_run, cache in env.gathered)
    assert len({id(cache) for *_, cache in env.gathered}) == 1, "one namespace cache per tick"
    assert env.redis.hashes["analyzer:review"]["shop/c"] == f"3:{int(env.redis.hashes['analyzer:review']['shop/c'].split(':')[1])}"


def test_sweep_budget_per_tick(monkeypatch):
    monkeypatch.setattr(main, "ANALYZER_REVIEW_APPS_PER_MIN", 1)
    apps = {f"a{i}": _app(f"a{i}") for i in range(5)}
    env = _reviewing(Env(monkeypatch, apps=apps), monkeypatch)
    _sweep(env, monkeypatch, [_summary(n) for n in apps])
    assert len(env.gathered) == 2  # ceil(1 app/min x 120 s tick / 60)


def test_sweep_yields_when_queue_not_empty(monkeypatch):
    env = _reviewing(Env(monkeypatch), monkeypatch)
    env.add("incident")
    listed, _ = _sweep(env, monkeypatch, [_summary("api")])
    assert listed == [] and env.gathered == []

    # A job queued mid-tick stops the sweep before the next app.
    env = _reviewing(Env(monkeypatch, apps={"api": _app("api"), "web": _app("web")}), monkeypatch)
    real = env.state.store.queue_len
    calls = []

    async def queue_len():
        calls.append(1)
        return 0 if len(calls) < 3 else await real() + 1

    monkeypatch.setattr(env.state.store, "queue_len", queue_len)
    _sweep(env, monkeypatch, [_summary("api"), _summary("web")])
    assert [name for _, name, _, _ in env.gathered] == ["api"]


def test_sweep_skips_inflight_app(monkeypatch):
    env = _reviewing(Env(monkeypatch), monkeypatch)
    env.redis.store["analyzer:inflight:shop:api"] = "1-1"
    _sweep(env, monkeypatch, [_summary("api"), _summary("web")])
    assert [name for _, name, _, _ in env.gathered] == ["web"]
    assert env.redis.store["analyzer:inflight:shop:api"] == "1-1", "another run's lock is never released"
    assert "analyzer:inflight:shop:web" not in env.redis.store


def test_sweep_skips_gone_or_moved_apps(monkeypatch):
    env = _reviewing(Env(monkeypatch, apps={"api": _app("api", namespace="elsewhere")}), monkeypatch)
    _sweep(env, monkeypatch, [_summary("api"), _summary("ghost")])
    assert env.gathered == [] and not any(k.startswith("analyzer:inflight:") for k in env.redis.store)


def test_sweep_disabled_when_interval_zero_or_ai_disabled(monkeypatch):
    env = _reviewing(Env(monkeypatch, cfg=AnalyzerConfig(enabled=False)), monkeypatch)
    listed, _ = _sweep(env, monkeypatch, [_summary("api")])
    assert listed == [] and env.gathered == []
    env = _reviewing(Env(monkeypatch), monkeypatch)
    monkeypatch.setattr(main, "ANALYZER_REVIEW_INTERVAL_SEC", 0)
    listed, _ = _sweep(env, monkeypatch, [_summary("api")])
    assert listed == [] and env.gathered == []


def test_sweep_deletes_app_after_two_missing_listings(monkeypatch):
    env = _reviewing(Env(monkeypatch), monkeypatch)
    gone = "analyzer:shop:gone"
    env.put(AppInsights(version=1, lastRun=LastRun(status="done")), key=gone)
    env.redis.zsets["analyzer:index"] = {"shop/gone": 1_800_000_000_000, "shop/api": 1_800_000_000_000}
    env.redis.hashes["analyzer:usage"] = {"shop/gone": "{}"}
    env.redis.hashes["analyzer:review"] = {"shop/gone": "3:1800000000", "shop/api": "3:1800000000"}
    _, missing = _sweep(env, monkeypatch, [_summary("api")])
    assert gone in env.redis.store and missing == {"shop/gone": 1}
    _, missing = _sweep(env, monkeypatch, [_summary("api")], missing=missing)
    assert gone not in env.redis.store and missing == {}
    assert "shop/gone" not in env.redis.zsets["analyzer:index"]
    assert "shop/gone" not in env.redis.hashes["analyzer:usage"] | env.redis.hashes["analyzer:review"]
    # Back in the listing before the second miss: nothing is deleted.
    _, missing = _sweep(env, monkeypatch, [_summary("web")], missing={})
    _, missing = _sweep(env, monkeypatch, [_summary("api"), _summary("web")], missing=missing)
    assert missing == {} and "shop/api" in env.redis.zsets["analyzer:index"]


def test_sweep_skips_tick_when_exporter_unavailable(monkeypatch):
    env = _reviewing(Env(monkeypatch), monkeypatch)
    env.redis.hashes["analyzer:review"] = {"shop/old": "3:1"}
    _, missing = _sweep(env, monkeypatch, [], fail=exporter.ExporterUnavailable(503))
    assert missing == {} and env.gathered == []
    _, missing = _sweep(env, monkeypatch, [])
    assert missing == {} and env.gathered == [], "an empty list never reads as 'every app was deleted'"


def test_sweep_gc_index(monkeypatch):
    env = _reviewing(Env(monkeypatch), monkeypatch)
    env.redis.zsets["analyzer:index"] = {"shop/api": 1.0, "shop/expired": 2.0}
    env.redis.hashes["analyzer:review"] = {"shop/api": "3:1"}
    _sweep(env, monkeypatch, [_summary("api"), _summary("expired", namespace="other")])
    index = env.redis.zsets["analyzer:index"]
    assert index["shop/api"] > 1.0, "re-scored by this tick's write"
    assert "shop/expired" not in index, "a member older than the document TTL is dropped"


def test_sweep_review_failure_logged_and_next_app_reviewed(monkeypatch):
    env = _reviewing(Env(monkeypatch, apps={"api": exporter.ExporterUnavailable(503), "web": _app("web")}),
                     monkeypatch)
    _sweep(env, monkeypatch, [_summary("api"), _summary("web")])
    assert [name for _, name, _, _ in env.gathered] == ["web"]
    assert "analyzer:inflight:shop:api" not in env.redis.store


def test_review_log_has_no_names(monkeypatch):
    env = _reviewing(Env(monkeypatch, apps={"secret-app": _app("secret-app", namespace="secret-ns")}), monkeypatch,
                     factory=lambda name: _single_replica_inputs(name))
    lines = []
    monkeypatch.setattr(main.logger, "info", lambda fmt, *args: lines.append(fmt.format(*args)))
    _sweep(env, monkeypatch, [_summary("secret-app", namespace="secret-ns")])
    review_lines = [line for line in lines if line.startswith("review ")]
    assert len(review_lines) == 1 and "gets=0 findings=1" in review_lines[0]
    assert review_lines[0].endswith("incomplete=")
    assert not any("secret" in line for line in lines)
    assert any(line == "sweep due=1 reviewed=1 removed=0" for line in lines)


def test_review_loop_survives_errors(monkeypatch):
    env = Env(monkeypatch)
    ticks = []

    async def tick(state, missing, clock=None):
        ticks.append(missing)
        raise RuntimeError("boom")

    async def sleep(seconds):
        if len(ticks) >= 2:
            raise Drained

    monkeypatch.setattr(main, "review_tick", tick)
    with pytest.raises(Drained):
        asyncio.run(main.review_loop(env.state, sleep=sleep))
    assert len(ticks) == 2 and ticks[0] is ticks[1], "the missing-app counts survive across ticks"
