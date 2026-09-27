"""Checks for review.py: the read budget, completeness per family, run-cache reuse, the wall, usage samples.

The review may only resolve what it saw completely, so every failed, truncated,
timed-out or skipped read must leave its family incomplete. The API server is an
httpx.MockTransport; Redis is tests/fakes.FakeRedis; the exporter is monkeypatched.

Run: python -m pytest tests/test_review_cov.py
"""

import asyncio
import json
import os
import subprocess
import sys
from datetime import UTC, datetime

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import exporter  # noqa: E402
import insights  # noqa: E402
import recommendations  # noqa: E402
import review  # noqa: E402
from fakes import FakeRedis  # noqa: E402
from models import AppInsights, Insight, Run  # noqa: E402
from tools import k8s_tools  # noqa: E402
from tools.k8s_tools import K8s  # noqa: E402

HERE = os.path.dirname(__file__)


def _app(workloads=(("shop", "Deployment", "api"), ("shop-db", "StatefulSet", "db")), services=(("shop", "api"),),
         metrics=None):
    resources = [{"namespace": ns, "kind": k, "name": n} for ns, k, n in workloads]
    resources += [{"namespace": ns, "kind": "Service", "name": n} for ns, n in services]
    return {"name": "shop", "namespaces": {"items": [{"name": "shop"}, {"name": "shop-db"}, {"name": "infra"}]},
            "resources": resources, "metrics": metrics or {}}


def _workload(name):
    return {"metadata": {"name": name}, "spec": {"replicas": 2, "selector": {
        "matchLabels": {"app": name}, "matchExpressions": [{"key": "tier", "operator": "In", "values": ["a", "b"]},
                                                            {"key": "canary", "operator": "DoesNotExist"}]}}}


class Api:
    """A fake kube-apiserver: routes by path, records every request; `fail` maps a path fragment to a status."""

    def __init__(self, fail=None, cont=(), slow=(), services=None):
        self.requests: list[httpx.Request] = []
        self.fail = fail or {}
        self.cont, self.slow = cont, slow
        self.services = services if services is not None else [
            {"metadata": {"name": "api"}, "spec": {"selector": {"app": "api"}}}]

    async def __call__(self, request):
        self.requests.append(request)
        path = request.url.path
        for fragment, code in self.fail.items():
            if fragment in path:
                return httpx.Response(code)
        if any(fragment in path for fragment in self.slow):
            await asyncio.sleep(0.2)
        meta = {"continue": "tok"} if any(fragment in path for fragment in self.cont) else {}
        if "/services" in path:
            return httpx.Response(200, json={"metadata": meta, "items": self.services})
        if "/pods" in path:
            return httpx.Response(200, json={"metadata": meta, "items": [{"metadata": {"name": "p"}}]})
        if "/apis/apps/v1/" in path:
            return httpx.Response(200, json=_workload(path.rsplit("/", 1)[-1]))
        return httpx.Response(200, json={"metadata": meta, "items": []})

    def paths(self):
        return [r.url.path for r in self.requests]


@pytest.fixture(autouse=True)
def _env(tmp_path, monkeypatch):
    token = tmp_path / "token"
    token.write_text("t")
    monkeypatch.setattr(k8s_tools, "SA_TOKEN_PATH", str(token))
    plans = {"calls": 0, "fail": False}

    async def snapshot(client, now):
        plans["calls"] += 1
        if plans["fail"]:
            raise exporter.ExporterUnavailable(503)
        return [{"id": "p1"}], {"e1": "production"}

    monkeypatch.setattr(exporter, "plans_snapshot", snapshot)
    return plans


def _gather(api=None, app=None, run=None, doc=None, redis=None, clock=None, k8s=True, **kw):
    app = app or _app()
    run = run or Run("shop", "shop", app, ["infra"])
    redis = redis or FakeRedis()
    api = api or Api()

    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(api), base_url="https://k8s") as client:
            extra = {"clock": clock} if clock else {}
            return await review.gather(app, doc or AppInsights(), run, K8s(client) if k8s else None, None, redis,
                                       **kw, **extra)

    return asyncio.run(go()), api


def test_gather_reads_budget():
    inputs, api = _gather()
    # 4 lists x 2 namespaces (infra is excluded) + (GET + pods) x 2 workloads + 1 selector Service.
    assert inputs.gets == 4 * 2 + 2 * 2 + 1 == len(api.requests)
    assert inputs.complete == {"W", "S", "P", "H", "N", "U", "A", "D", "X"}
    assert inputs.namespaces == ["shop", "shop-db"] and not any("/infra/" in p for p in api.paths())
    assert inputs.excluded == ["infra"], "the rules resolve the cards of a namespace excluded later"
    assert set(inputs.workloads) == {("shop", "deployment/api"), ("shop-db", "statefulset/db")}
    pods = [r for r in api.requests if r.url.path.endswith("/pods")]
    assert pods[0].url.params["labelSelector"] == "app=api,tier in (a,b),!canary"
    assert pods[0].url.params["limit"] == "500"
    probe = pods[-1]
    assert (probe.url.params["labelSelector"], probe.url.params["limit"]) == ("app=api", "1")
    assert inputs.selector_pods == {("shop", "api"): 1}
    assert all(r.method == "GET" for r in api.requests), "the review never writes to the cluster"

    # This run's status reads are reused: no workload GET, no pod list for them.
    run = Run("shop", "shop", _app(), ["infra"])
    run.spec_cache[("shop", "deployment/api")] = _workload("api")
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": "p"}}]
    run.spec_cache[("shop-db", "statefulset/db")] = _workload("db")
    run.pods_cache[("shop-db", "statefulset/db")] = [{"metadata": {"name": "q"}}]
    inputs, api = _gather(run=run)
    assert inputs.gets == 4 * 2 + 1 and not any("/apis/apps/" in p for p in api.paths())
    assert inputs.workloads[("shop", "deployment/api")].pods == [{"metadata": {"name": "p"}}]

    # Namespace lists are shared across the apps of one sweep tick.
    cache: dict = {}
    first, _ = _gather(ns_cache=cache)
    second, api = _gather(ns_cache=cache)
    assert first.gets == 13 and second.gets == 5 and "S" in second.complete


def test_gather_403_marks_family_incomplete():
    inputs, _ = _gather(Api(fail={"poddisruptionbudgets": 403}))
    assert "P" not in inputs.complete and {"S", "H", "N", "W"} <= inputs.complete
    assert inputs.items("P", "shop") is None
    inputs, _ = _gather(Api(fail={"/statefulsets/": 403}))
    assert "W" not in inputs.complete and ("shop", "deployment/api") in inputs.workloads
    inputs, _ = _gather(Api(fail={"/pods": 500}))
    assert "W" not in inputs.complete and inputs.selector_pods == {("shop", "api"): None}


def _pdb_card(namespace):
    card_id = insights.recommendation_id("shop", "shop", "deployment/api", "reliability.no_pdb", namespace)
    return Insight(id=card_id, kind="reliability", subject="deployment/api", status="open", category="recommendation",
                   reason="reliability.no_pdb", params={"namespace": namespace})


def _review(api, app, doc, run=None):
    inputs, api = _gather(api, app=app, doc=doc, run=run)
    findings, evaluated = recommendations.evaluate(inputs, datetime.now(UTC))
    return inputs, insights.merge_recommendations(doc, findings, evaluated, "2026-09-25T12:00:00Z", "shop", "shop"), api


def test_gather_404_workload_is_gone():
    """Discovery keeps an Application's last resources list when its last workload is deleted: a 404 on the
    workload's GET means gone, the review stays complete and the deleted workload's cards resolve."""
    both = _app(workloads=(("shop", "Deployment", "api"), ("shop-db", "Deployment", "api")))
    doc = AppInsights(insights=[_pdb_card("shop"), _pdb_card("shop-db")])
    inputs, stats, _ = _review(Api(fail={"shop-db/deployments/api": 404}), both, doc)
    assert "W" in inputs.complete and inputs.gone == {("shop-db", "deployment", "api")}
    assert set(inputs.workloads) == {("shop", "deployment/api")}
    # Only the deleted copy resolves: the same workload in the other namespace still has no PDB.
    assert stats.resolved == [doc.insights[1].id] and doc.insights[1].status == "resolved"

    # The app's last workload deleted: nothing left to read, and the review is complete.
    doc = AppInsights(insights=[_pdb_card("shop")])
    inputs, stats, _ = _review(Api(fail={"/deployments/api": 404}), _app(workloads=(("shop", "Deployment", "api"),)),
                               doc)
    assert "W" in inputs.complete and inputs.workloads == {} and stats.resolved == [doc.insights[0].id]

    # Any other failure is no verdict: the family stays incomplete and nothing resolves.
    for code in (403, 500):
        doc = AppInsights(insights=[_pdb_card("shop-db")])
        inputs, stats, _ = _review(Api(fail={"shop-db/deployments/api": code}), both, doc)
        assert "W" not in inputs.complete and inputs.gone == set() and stats.resolved == []

    # This run's status read already answered 404: no second GET.
    run = Run("shop", "shop", both, ["infra"])
    run.gone.add(("shop-db", "deployment/api"))
    inputs, _, api = _review(None, both, AppInsights(), run)
    assert inputs.gone == {("shop-db", "deployment", "api")} and "W" in inputs.complete
    assert not any(p.endswith("shop-db/deployments/api") for p in api.paths())


def test_gather_continue_token_incomplete():
    inputs, _ = _gather(Api(cont=("networkpolicies",)))
    assert "N" not in inputs.complete and "S" in inputs.complete


def test_gather_timeout_and_wall(monkeypatch):
    monkeypatch.setattr(review, "TOOL_TIMEOUT_S", 0.05)
    inputs, _ = _gather(Api(slow=("horizontalpodautoscalers",)))
    assert "H" not in inputs.complete and {"S", "P", "N", "W"} <= inputs.complete

    # The wall: after the first reads the clock is past it, every later family is incomplete.
    ticks = iter([0.0] * 3 + [10_000.0] * 100)
    inputs, api = _gather(clock=lambda: next(ticks))
    assert "S" in inputs.complete and not ({"P", "H", "N", "W"} & inputs.complete)
    assert inputs.gets == 2 == len(api.requests)


def test_gather_without_cluster_access(_env):
    _env["fail"] = True
    inputs, api = _gather(k8s=False)
    assert inputs.complete == {"A", "D", "U"} and api.requests == [] and inputs.plans is None


def test_events_family_only_in_run():
    run = Run("shop", "shop", _app(), ["infra"])
    run.events_cache.append({"reason": "Unhealthy"})
    assert "E" not in _gather(run=run, in_run=True)[0].complete, "events never read by this run"
    run.calls["get_recent_events"] = 1
    inputs, _ = _gather(run=run, in_run=True)
    assert "E" in inputs.complete and inputs.events == [{"reason": "Unhealthy"}]
    assert "E" not in _gather(run=run)[0].complete, "a sweep review has no events"
    run.truncated = True
    assert "E" not in _gather(run=run, in_run=True)[0].complete


def test_workload_cap_prefers_incident_workloads(monkeypatch):
    monkeypatch.setattr(review, "ANALYZER_REVIEW_WORKLOADS_MAX", 1)
    doc = AppInsights(insights=[Insight(id="i", subject="statefulset/db", status="open", kind="oom"),
                                Insight(id="r", subject="deployment/api", status="open", kind="reliability",
                                        category="recommendation")])
    inputs, _ = _gather(doc=doc)
    assert set(inputs.workloads) == {("shop-db", "statefulset/db")} and inputs.capped
    assert "W" in inputs.complete, "the cap is not a failed read"
    inputs, _ = _gather()
    assert set(inputs.workloads) == {("shop", "deployment/api")}


def _usage(ts, cpu="12m", mem="45.23Mi", available=True):
    return {"resourceName": "api", "resourceKind": "Deployment", "namespace": "shop", "usage": {
        "available": available, "timestamp": ts, "resources": {"usagePerInstance": [
            {"name": "p1", "containers": [{"name": "app", "cpu": cpu, "memory": mem}]},
            {"name": "p2", "containers": [{"name": "app", "cpu": "0.05", "memory": "10Mi"},
                                          {"name": "side", "cpu": "bogus", "memory": "1Mi"}]}]}}}


def test_usage_sample_appended_deduped_trimmed(monkeypatch):
    redis = FakeRedis()
    app = _app(metrics={"workloads": [_usage("2026-09-24T10:00:00Z")]})
    inputs, _ = _gather(app=app, redis=redis)
    (samples,) = inputs.usage.values()
    assert list(inputs.usage) == [("shop", "deployment/api")]
    assert samples[0]["containers"] == {"app": {"cpu_m": 50, "mem_b": 47427093}, "side": {"cpu_m": None, "mem_b": 1048576}}
    # The same timestamp again: no new sample, no write.
    redis.hashes["analyzer:usage"]["shop/shop"] = json.dumps(json.loads(redis.hashes["analyzer:usage"]["shop/shop"]))
    inputs, _ = _gather(app=app, redis=redis)
    assert len(inputs.usage[("shop", "deployment/api")]) == 1
    # Unavailable or undated usage adds nothing; newer samples append and the oldest drop past USAGE_SAMPLES_MAX.
    for bad in (_usage("2026-09-24T11:00:00Z", available=False), _usage("not a time"), _usage(None),
                {**_usage("2026-09-24T11:00:00Z"), "usage": {"available": True, "timestamp": "2026-09-24T11:00:00Z"}}):
        _gather(app=_app(metrics={"workloads": [bad]}), redis=redis)
    monkeypatch.setattr(review, "USAGE_SAMPLES_MAX", 3)
    for hour in range(11, 16):
        _gather(app=_app(metrics={"workloads": [_usage(f"2026-09-24T{hour}:00:00Z")]}), redis=redis)
    stored = json.loads(redis.hashes["analyzer:usage"]["shop/shop"])["shop/deployment/api"]
    assert [s["t"] for s in stored] == [datetime(2026, 9, 24, h, tzinfo=UTC).timestamp() for h in (13, 14, 15)]


def test_usage_family_needs_min_samples_and_span(monkeypatch):
    monkeypatch.setattr(review, "ANALYZER_USAGE_MIN_SAMPLES", 3)
    monkeypatch.setattr(review, "ANALYZER_USAGE_MIN_SPAN_SEC", 100)
    assert review.sufficient([{"t": 0}, {"t": 50}, {"t": 100}])
    assert not review.sufficient([{"t": 0}, {"t": 100}]), "too few samples"
    assert not review.sufficient([{"t": 0}, {"t": 50}, {"t": 99}]), "too short a span"
    assert not review.sufficient([])


def test_usage_redis_failure_leaves_family_incomplete():
    class Down(FakeRedis):
        async def hget(self, name, key):
            from redis import ConnectionError as RedisConnectionError
            raise RedisConnectionError("down")

    inputs, _ = _gather(redis=Down())
    assert "U" not in inputs.complete and "W" in inputs.complete


def test_selector_string_and_service_probe_skips():
    assert review.selector_string(None) == ""
    assert review.selector_string({"matchExpressions": [{"key": "a", "operator": "NotIn", "values": ["x"]},
                                                        {"key": "b", "operator": "Exists"},
                                                        {"key": "c", "operator": "Bogus"}]}) == "a notin (x),b"
    # A selector-less (or unknown) Service is never probed.
    inputs, api = _gather(Api(services=[{"metadata": {"name": "api"}, "spec": {}}]))
    assert inputs.selector_pods == {} and inputs.gets == 12
    inputs, _ = _gather(app=_app(services=(("infra", "x"), ("shop", "gone"))))
    assert inputs.selector_pods == {}


def test_invalid_production_pattern_fails_import():
    env = {**os.environ, "ANALYZER_PRODUCTION_PATTERN": "(unclosed"}
    code = "import config"
    proc = subprocess.run([sys.executable, "-c", code], cwd=os.path.join(HERE, ".."), env=env, capture_output=True)
    assert proc.returncode != 0 and b"ANALYZER_PRODUCTION_PATTERN" not in proc.stdout
    ok = subprocess.run([sys.executable, "-c", code], cwd=os.path.join(HERE, ".."),
                        env={**os.environ, "ANALYZER_PRODUCTION_PATTERN": "^live$"}, capture_output=True)
    assert ok.returncode == 0
