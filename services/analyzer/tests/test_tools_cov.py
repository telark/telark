"""Checks for the four read-only tools: validation, binding, caps, filtering, failures.

The tools are the model's only window into the cluster, so every guard here is a
trust boundary: arguments come from the model, results go back into its prompt.
No test touches a real cluster: the API server is an httpx.MockTransport.

Run: python -m pytest tests/test_tools_cov.py
"""

import asyncio
import json
import os
import sys
from datetime import UTC, datetime, timedelta

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import tools  # noqa: E402
from models import EventsArgs, Run  # noqa: E402
from tools import SPECS, fit, invoke  # noqa: E402
from tools import k8s_tools  # noqa: E402
from tools.k8s_tools import K8s  # noqa: E402

TOKEN = "token-v1"


def _ts(minutes_ago):
    return (datetime.now(UTC) - timedelta(minutes=minutes_ago)).strftime("%Y-%m-%dT%H:%M:%SZ")


def _app(resources=None):
    return {
        "name": "shop",
        "health": {"status": "degraded", "reason": "1/2 replicas ready", "readyReplicas": 1, "totalReplicas": 2},
        "namespaces": {"total": 3, "items": [{"name": "shop"}, {"name": "shop-db"}, {"name": "infra"}]},
        "managed": {"by": "helm", "chart": "shop", "version": "1.0.0"},
        "resources": resources if resources is not None else [
            {"namespace": "shop", "kind": "Deployment", "name": "api"},
            {"namespace": "shop-db", "kind": "StatefulSet", "name": "api-worker"},
            {"namespace": "infra", "kind": "DaemonSet", "name": "agent"},
            {"namespace": "shop", "kind": "Service", "name": "api"},
        ],
        "snapshots": [{"id": f"s{i}", "generation": i} for i in range(1, 5)],
        "history": {
            "generation": 7,
            "hasDrift": True,
            "changeLog": [
                {"generation": 5, "detectedAt": "t5", "changeClass": "config", "severity": "low",
                 "isIncident": False, "isRecovery": False, "changes": []},
                {"generation": 7, "detectedAt": "t7", "changeClass": "image", "severity": "high",
                 "isIncident": True, "isRecovery": False,
                 "changes": [{"field": f"f{i}", "changeType": "updated", "oldValue": "o" * 200, "newValue": None}
                             for i in range(9)]},
                {"generation": 6, "detectedAt": "t6", "changeClass": "scale", "severity": "medium",
                 "isIncident": False, "isRecovery": True, "changes": []},
            ],
        },
    }


def _run(resources=None):
    return Run(namespace="shop", name="shop", app=_app(resources), excluded=["infra"])


def _deployment(ready=1):
    return {
        "spec": {"replicas": 2, "selector": {"matchLabels": {"app": "api", "tier": "web"}},
                 "template": {"spec": {"containers": [{"image": "registry.io/team/api:1.2.3"}]}}},
        "status": {"updatedReplicas": 2, "readyReplicas": ready,
                   "conditions": [{"type": "Available", "status": "False", "reason": "MinimumReplicasUnavailable",
                                   "message": "ignored"}]},
    }


def _pod(name, restarts, waiting=None, terminated=None):
    status = {"restartCount": restarts, "state": {}, "lastState": {}}
    if waiting:
        status["state"]["waiting"] = {"reason": waiting}
    if terminated:
        status["lastState"]["terminated"] = {"reason": terminated, "exitCode": 137, "finishedAt": "t"}
    return {"metadata": {"name": name}, "status": {"phase": "Running", "containerStatuses": [status]}}


def _event(kind, name, reason="BackOff", minutes_ago=1, message="Back-off restarting failed container", count=1):
    return {"involvedObject": {"kind": kind, "name": name}, "reason": reason, "message": message,
            "count": count, "firstTimestamp": _ts(minutes_ago + 1), "lastTimestamp": _ts(minutes_ago)}


class Api:
    """A fake kube-apiserver: routes by path, records every request."""

    def __init__(self, events=None, deployment=None, pods=None):
        self.requests: list[httpx.Request] = []
        self.events = events or {}
        self.deployment = deployment or _deployment()
        self.pods = pods if pods is not None else [_pod("api-7d9f8b6c5-aaaaa", 0), _pod("api-7d9f8b6c5-bbbbb", 4,
                                                                                         "CrashLoopBackOff", "OOMKilled")]

    def __call__(self, request):
        self.requests.append(request)
        path = request.url.path
        if path.endswith("/events"):
            ns = path.split("/")[4]
            return httpx.Response(200, json={"items": self.events.get(ns, [])})
        if path.endswith("/pods"):
            return httpx.Response(200, json={"items": self.pods})
        if "/apis/apps/v1/" in path:
            return httpx.Response(200, json=self.deployment)
        return httpx.Response(404, text="unexpected")


@pytest.fixture(autouse=True)
def _token(tmp_path, monkeypatch):
    path = tmp_path / "token"
    path.write_text(TOKEN + "\n")
    monkeypatch.setattr(k8s_tools, "SA_TOKEN_PATH", str(path))
    return path


def _invoke(run, name, arguments, handler=None, k8s=True):
    async def go():
        if not k8s:
            return await invoke(run, name, arguments, None)
        transport = httpx.MockTransport(handler or Api())
        async with httpx.AsyncClient(transport=transport, base_url="https://kubernetes.default.svc") as client:
            return await invoke(run, name, arguments, K8s(client))

    return asyncio.run(go())


def _content(result):
    return json.loads(result.content)


def test_invoke_rejects_unknown_field():
    run = _run()
    result = _invoke(run, "get_change_history", {"limit": 5, "bogus": "secret-input-value"})
    assert result.error == "invalid_arguments"
    assert "secret-input-value" not in result.content, "the error must never echo the model's input"
    assert _content(result)["error"] == "invalid_arguments"

    result = _invoke(run, "get_change_history", {"limit": 11})
    assert result.error == "invalid_arguments"
    assert "11" not in result.content

    assert _invoke(run, "get_workload_status", {"kind": "pod", "name": "api"}).error == "invalid_arguments"
    assert _invoke(run, "get_app_overview", "not json").error == "invalid_arguments"
    assert _invoke(run, "get_app_overview", ["a list"]).error == "invalid_arguments"
    assert _invoke(run, "delete_deployment", {}).error == "unknown_tool"
    assert run.tool_calls == 0 and not run.calls, "rejected calls never reach a tool"

    ok = _invoke(run, "get_change_history", json.dumps({"limit": 1}))
    assert ok.error == "" and run.tool_calls == 1


def test_workload_binding():
    run = _run()
    result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "billing"})
    assert result.error == "unknown_workload"
    # A real workload of the app, asked under the wrong kind, is still not bound.
    assert _invoke(run, "get_workload_status", {"kind": "daemonset", "name": "api"}).error == "unknown_workload"

    api = Api()
    result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}, api)
    assert result.error == ""
    body = _content(result)
    assert (body["desired"], body["updated"], body["ready"]) == (2, 2, 1)
    assert body["conditions"] == [{"type": "Available", "status": "False", "reason": "MinimumReplicasUnavailable"}]
    assert body["pods"][0]["name"] == "api-7d9f8b6c5-bbbbb", "pods sort by restarts desc"
    assert body["pods"][0]["waitingReason"] == "CrashLoopBackOff"
    assert body["pods"][0]["lastTerminated"] == {"reason": "OOMKilled", "exitCode": 137, "at": "t"}
    assert body["images"] == ["api:1.2.3"]
    assert result.refs == ["workload:deployment/api"]
    assert "workload:deployment/api" in run.refs
    assert run.status_cache[("shop", "deployment/api")]["ready"] == 1
    paths = [r.url.path for r in api.requests]
    assert paths == ["/apis/apps/v1/namespaces/shop/deployments/api", "/api/v1/namespaces/shop/pods"]
    assert api.requests[1].url.params["labelSelector"] == "app=api,tier=web"


def test_same_workload_in_two_namespaces_binds_by_namespace():
    run = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"},
                {"namespace": "shop-db", "kind": "Deployment", "name": "api"}])
    args = {"kind": "deployment", "name": "api"}
    assert _invoke(run, "get_workload_status", {**args, "namespace": "infra"}).error == "unknown_workload"
    api = Api()
    _invoke(run, "get_workload_status", {**args, "namespace": "shop-db"}, api)
    _invoke(run, "get_workload_status", args, api)
    assert [r.url.path for r in api.requests if "/apis/apps/" in r.url.path] == [
        "/apis/apps/v1/namespaces/shop-db/deployments/api", "/apis/apps/v1/namespaces/shop/deployments/api"]
    assert set(run.status_cache) == {("shop-db", "deployment/api"), ("shop", "deployment/api")}
    # Events carry their namespace for the rules; the model's payload stays as it was.
    api = Api(events={n: [_event("Pod", "api-7d9f8b6c5-x2k9p")] for n in ("shop", "shop-db")})
    body = _content(_invoke(run, "get_recent_events", {}, api))
    assert len(body["events"]) == 2 and all("namespace" not in e for e in body["events"])
    assert sorted(e["namespace"] for e in run.events_cache) == ["shop", "shop-db"]


def test_daemonset_counts_and_no_selector():
    run = Run(namespace="shop", name="shop", app=_app([{"namespace": "shop", "kind": "DaemonSet", "name": "agent"}]),
              excluded=[])
    ds = {"spec": {"selector": {}, "template": {"spec": {"containers": [{"image": "agent"}]}}},
          "status": {"desiredNumberScheduled": 3, "updatedNumberScheduled": 2, "numberReady": 1}}
    api = Api(deployment=ds)
    body = _content(_invoke(run, "get_workload_status", {"kind": "daemonset", "name": "agent"}, api))
    assert (body["desired"], body["updated"], body["ready"]) == (3, 2, 1)
    assert body["pods"] == [] and body["images"] == ["agent"]
    assert len(api.requests) == 1, "no matchLabels: never list every pod of the namespace"


def test_event_name_filter():
    names = ["api-7d9f8b6c5-x2k9p", "api-gateway-7d9f8b6c5-x2k9p", "api-worker-0", "api-7d9f8b6c5", "api-x2k9p"]
    events = {"shop": [_event("Pod", n) for n in names] + [_event("Deployment", "api", reason="ProgressDeadline"),
                                                         _event("ReplicaSet", "api-7d9f8b6c5", "FailedCreate")]}

    deploy_only = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"}])
    body = _content(_invoke(deploy_only, "get_recent_events", {}, Api(events)))
    kept = {e["object"] for e in body["events"]}
    assert kept == {"Pod/api-7d9f8b6c5-x2k9p", "Deployment/api", "ReplicaSet/api-7d9f8b6c5"}

    with_sts = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"},
                     {"namespace": "shop", "kind": "StatefulSet", "name": "api-worker"},
                     {"namespace": "shop", "kind": "DaemonSet", "name": "api"}])
    body = _content(_invoke(with_sts, "get_recent_events", {}, Api(events)))
    kept = {e["object"] for e in body["events"]}
    assert "Pod/api-worker-0" in kept
    assert "Pod/api-x2k9p" in kept, "DaemonSet pod shape"
    assert "Pod/api-gateway-7d9f8b6c5-x2k9p" not in kept


def test_event_window_dedupe_and_order():
    run = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"}])
    pod = "api-7d9f8b6c5-x2k9p"
    series = {"involvedObject": {"kind": "Pod", "name": pod}, "reason": "Unhealthy", "message": "probe",
              "eventTime": _ts(90), "series": {"count": 4, "lastObservedTime": _ts(2)}}
    older, newer = _event("Pod", pod, minutes_ago=5, count=2), _event("Pod", pod, minutes_ago=3, count=3)
    events = {"shop": [
        older,
        newer,
        _event("Pod", pod, reason="Old", minutes_ago=45),
        {"involvedObject": {"kind": "Pod", "name": pod}, "reason": "Undated", "message": "x"},
        series,
        _event("Pod", pod, reason="Long", minutes_ago=1, message="m" * 400),
    ]}
    api = Api(events)
    result = _invoke(run, "get_recent_events", {"sinceMinutes": 30, "warningsOnly": False}, api)
    body = _content(result)
    assert [e["reason"] for e in body["events"]] == ["Long", "Unhealthy", "BackOff"], "sorted by last desc"
    backoff = body["events"][2]
    assert backoff["count"] == 5
    assert (backoff["first"], backoff["last"]) == (older["firstTimestamp"], newer["lastTimestamp"])
    assert body["events"][1]["count"] == 4
    assert len(body["events"][0]["message"]) == 160
    assert f"BackOff@Pod/{pod}@{newer['lastTimestamp']}" in result.refs
    assert "fieldSelector" not in api.requests[0].url.params
    assert api.requests[0].url.params["limit"] == "200"
    assert len(run.events_cache) == 3


def test_result_truncation():
    payload = {"events": [{"message": "é" * 300} for _ in range(20)], "note": "x"}
    content, truncated = fit(payload, "events")
    assert truncated is True
    assert len(content.encode()) <= 2048
    assert json.loads(content)["events"], "items are dropped whole, never cut inside a string"
    assert fit({"events": []}, "events") == ('{"events":[]}', False)

    run = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"}])
    events = {"shop": [_event("Pod", "api-7d9f8b6c5-x2k9p", reason=f"R{i}", message="m" * 150, minutes_ago=i)
                       for i in range(20)]}
    result = _invoke(run, "get_recent_events", {}, Api(events))
    body = _content(result)
    assert result.truncated is True
    assert len(result.content.encode()) <= 2048
    assert 0 < len(body["events"]) < 12
    assert len(result.refs) == len(body["events"]), "refs only for what the model actually sees"
    assert len(run.events_cache) == 12, "the resolve cache keeps the full deduped list"
    assert run.truncated is False, "a size cut is not a failed fetch"


def test_call_caps():
    run = _run()
    for _ in range(3):
        assert _invoke(run, "get_app_overview", {}).error == "", "a repeat overview is served, not capped"
    for _ in range(2):
        assert _invoke(run, "get_change_history", {}).error == ""
    assert _invoke(run, "get_change_history", {}).error == "call_cap_exceeded"
    for _ in range(3):
        assert _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}).error == ""
    assert _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}).error == "call_cap_exceeded"
    for _ in range(2):
        assert _invoke(run, "get_recent_events", {}).error == ""
    assert _invoke(run, "get_recent_events", {}).error == "call_cap_exceeded"
    assert run.tool_calls == 10


def test_overview_and_history():
    run = _run()
    result = _invoke(run, "get_app_overview", {})
    body = _content(result)
    assert body["health"] == {"status": "degraded", "reason": "1/2 replicas ready", "ready": 1, "total": 2}
    assert body["namespaces"] == ["shop", "shop-db"]
    assert body["managedBy"] == "helm"
    assert body["workloads"] == [{"kind": "deployment", "name": "api", "namespace": "shop"},
                                 {"kind": "statefulset", "name": "api-worker", "namespace": "shop-db"}]
    assert (body["generation"], body["hasDrift"]) == (7, True)
    assert body["lastChange"] == {"generation": 7, "changeClass": "image", "severity": "high",
                                  "isIncident": True, "isRecovery": False}
    assert body["snapshotIds"] == ["s2", "s3", "s4"]
    assert set(result.refs) == {"workload:deployment/api", "workload:statefulset/api-worker",
                                "snap:s2", "snap:s3", "snap:s4"}

    body = _content(_invoke(run, "get_change_history", {"limit": 2}))
    assert [c["generation"] for c in body["changes"]] == [7, 6]
    assert len(body["changes"][0]["changes"]) == 6
    assert body["changes"][0]["changes"][0]["oldValue"] == "o" * 80
    assert body["changes"][0]["changes"][0]["newValue"] is None
    incidents = _invoke(run, "get_change_history", {"onlyIncidents": True})
    assert [c["generation"] for c in _content(incidents)["changes"]] == [7]
    assert incidents.refs == ["gen:7"]

    empty = Run(namespace="n", name="n", app={}, excluded=[])
    body = _content(_invoke(empty, "get_app_overview", {}))
    assert body["lastChange"] is None and body["workloads"] == [] and body["snapshotIds"] == []


def test_excluded_namespaces():
    run = _run()
    assert ("infra", "daemonset/agent") not in run.workloads, "a workload in an excluded namespace is never bound"
    assert _invoke(run, "get_workload_status", {"kind": "daemonset", "name": "agent"}).error == "unknown_workload"
    assert _invoke(run, "get_recent_events", {"namespace": "infra"}).error == "unknown_namespace"
    assert _invoke(run, "get_recent_events", {"namespace": "kube-system"}).error == "unknown_namespace"

    api = Api()
    assert _invoke(run, "get_recent_events", {}, api).error == ""
    paths = [r.url.path for r in api.requests]
    assert paths == ["/api/v1/namespaces/shop/events", "/api/v1/namespaces/shop-db/events"]
    assert all(r.url.params["fieldSelector"] == "type=Warning" for r in api.requests)

    api = Api()
    assert _invoke(run, "get_recent_events", {"namespace": "shop-db"}, api).error == ""
    assert [r.url.path for r in api.requests] == ["/api/v1/namespaces/shop-db/events"]

    many = Run(namespace="n", name="n", excluded=[],
               app={"namespaces": {"items": [{"name": f"ns{i}"} for i in range(8)]}})
    api = Api()
    _invoke(many, "get_recent_events", {}, api)
    assert len(api.requests) == 5, "at most 5 namespaces per call"
    assert EventsArgs().namespace == ""


def test_timeout_sets_truncated(monkeypatch):
    monkeypatch.setattr(tools, "TOOL_TIMEOUT_S", 0.01)

    async def slow(request):
        await asyncio.sleep(1)
        return httpx.Response(200, json={})

    run = _run()
    result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}, slow)
    assert result.error == "timeout"
    assert run.truncated is True
    assert run.tool_calls == 1


def test_k8s_error_is_tool_error():
    run = _run()
    result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"},
                     lambda request: httpx.Response(404, text="deployments.apps \"api\" not found SECRET-BODY"))
    assert result.error == "k8s_error"
    assert _content(result) == {"error": "k8s_error", "detail": 404}
    assert "SECRET-BODY" not in result.content
    # A 404 is a deleted workload the Application still lists: gone, not a read that fell short.
    assert run.truncated is False and run.gone == {("shop", "deployment/api")}

    for code in (403, 500):
        run = _run()
        result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"},
                         lambda request, code=code: httpx.Response(code))
        assert _content(result) == {"error": "k8s_error", "detail": code}
        assert run.truncated is True and run.gone == set()

    def refused(request):
        raise httpx.ConnectError("connection refused to 10.0.0.1 SECRET-TEXT")

    run = _run()
    result = _invoke(run, "get_recent_events", {}, refused)
    assert _content(result) == {"error": "k8s_error", "detail": "ConnectError"}
    assert run.truncated is True


def test_transport_timeout_is_tool_timeout():
    def read_timeout(request):
        raise httpx.ReadTimeout("read timed out")

    run = _run()
    result = _invoke(run, "get_recent_events", {}, read_timeout)
    assert result.error == "timeout"
    assert run.truncated is True


def test_k8s_unavailable():
    run = _run()
    assert _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}, k8s=False).error == \
        "k8s_unavailable"
    assert run.truncated is True
    run = _run()
    assert _invoke(run, "get_recent_events", {}, k8s=False).error == "k8s_unavailable"
    assert run.truncated is True
    assert _invoke(run, "get_app_overview", {}, k8s=False).error == "", "app tools need no cluster access"


def test_k8s_get_only_with_token(_token):
    api = Api()

    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(api), base_url="https://kubernetes.default.svc") as c:
            k8s = K8s(c)
            await k8s.get("/api/v1/namespaces/shop/pods")
            _token.write_text("token-v2")
            await k8s.get("/api/v1/namespaces/shop/pods")
            return [n for n in dir(k8s) if not n.startswith("_")]

    public = asyncio.run(go())
    assert public == ["get"], "GET is the only request method the analyzer has"
    assert {r.method for r in api.requests} == {"GET"}
    assert [r.headers["Authorization"] for r in api.requests] == [f"Bearer {TOKEN}", "Bearer token-v2"], \
        "the projected token is re-read on every call"

    run = _run()
    api = Api()
    _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}, api)
    _invoke(run, "get_recent_events", {}, api)
    assert api.requests and {r.method for r in api.requests} == {"GET"}


def test_specs_are_strict_schemas():
    assert [s["function"]["name"] for s in SPECS] == [
        "get_app_overview", "get_change_history", "get_workload_status", "get_recent_events"]

    def titles(node):
        if isinstance(node, dict):
            return ("title" in node) + sum(titles(v) for v in node.values())
        if isinstance(node, list):
            return sum(titles(v) for v in node)
        return 0

    for spec in SPECS:
        assert spec["type"] == "function"
        assert spec["function"]["description"]
        params = spec["function"]["parameters"]
        assert params["additionalProperties"] is False
        assert params["type"] == "object"
    assert titles(SPECS) == 0
    kind = SPECS[2]["function"]["parameters"]["properties"]["kind"]
    assert kind["enum"] == ["deployment", "statefulset", "daemonset"]
    assert SPECS[1]["function"]["parameters"]["properties"]["limit"]["maximum"] == 10


# ---- richer pod facts, caches for the review (S3) -------------------------------------------
NOT_FOUND = ('Failed to pull image "registry.k8s.io/pause:does-not-exist": rpc error: code = NotFound desc = failed '
             'to pull and unpack image "registry.k8s.io/pause:does-not-exist": failed to resolve reference '
             '"registry.k8s.io/pause:does-not-exist": registry.k8s.io/pause:does-not-exist: not found')


def _container(name, restarts=0, waiting=None, message=None, terminated=None, code=None):
    status = {"name": name, "restartCount": restarts, "state": {}, "lastState": {}}
    if waiting:
        status["state"]["waiting"] = {"reason": waiting, "message": message}
    if terminated:
        status["lastState"]["terminated"] = {"reason": terminated, "exitCode": code, "finishedAt": "t"}
    return status


def test_pod_summary_reports_waiting_init_container():
    pod = {"metadata": {"name": "p"}, "spec": {"nodeName": "n1"},
           "status": {"phase": "Pending",
                      "initContainerStatuses": [_container("init", 3, "CrashLoopBackOff", "back-off", "Error", 1)],
                      "containerStatuses": [_container("app", 0, "PodInitializing")]}}
    summary = k8s_tools._pod_summary(pod)
    assert (summary["container"], summary["init"], summary["waitingReason"]) == ("init", True, "CrashLoopBackOff")
    assert summary["lastTerminated"] == {"reason": "Error", "exitCode": 1, "at": "t"}
    assert summary["restarts"] == 3 and summary["waitingMessage"] == "back-off"


def test_pod_summary_exit_code_matches_container():
    # The sidecar was OOMKilled long ago and runs; the app container crash-loops with exit 127: report the app.
    pod = {"metadata": {"name": "p"},
           "status": {"phase": "Running", "containerStatuses": [
               _container("sidecar", 1, terminated="OOMKilled", code=137),
               _container("app", 5, "CrashLoopBackOff", terminated="Error", code=127)]}}
    summary = k8s_tools._pod_summary(pod)
    assert (summary["container"], summary["init"]) == ("app", False)
    assert summary["lastTerminated"]["exitCode"] == 127 and summary["restarts"] == 6
    # Nothing waits: the first container with a last termination is reported.
    pod["status"]["containerStatuses"][1] = _container("app", 5)
    summary = k8s_tools._pod_summary(pod)
    assert (summary["container"], summary["lastTerminated"]["reason"]) == ("sidecar", "OOMKilled")
    assert summary["waitingReason"] is None


def test_pod_summary_node():
    pod = {"metadata": {"name": "p"}, "spec": {"nodeName": "ip-10-0-1-5"},
           "status": {"phase": "Running", "conditions": [
               {"type": "Ready", "status": "True"},
               {"type": "DisruptionTarget", "status": "True", "reason": "PreemptionByScheduler"}]}}
    summary = k8s_tools._pod_summary(pod)
    assert summary["node"] == "ip-10-0-1-5" and summary["disruption"] == "PreemptionByScheduler"
    assert summary["container"] is None and summary["lastTerminated"] is None
    assert k8s_tools._pod_summary({})["node"] is None


def test_workload_status_limits_and_caches():
    run = _run()
    deployment = _deployment()
    deployment["spec"]["template"]["spec"]["containers"] = [
        {"name": "api", "image": "registry.io/team/api:1.2.3", "resources": {"limits": {"memory": "64Mi"}}},
        {"name": "side", "image": "busybox"}]
    pending = {"metadata": {"name": "api-7d9f8b6c5-ccccc"}, "status": {"phase": "Pending"}}
    waiting = {"metadata": {"name": "api-7d9f8b6c5-ddddd"}, "status": {"phase": "Pending", "containerStatuses": [
        _container("api", 0, "ErrImagePull", "rpc error: pull access denied")]}}
    api = Api(deployment=deployment, pods=[pending, waiting])
    result = _invoke(run, "get_workload_status", {"kind": "deployment", "name": "api"}, api)
    body = _content(result)
    assert body["limits"] == {"api": "64Mi"}
    assert all("waitingMessage" not in p and "disruption" not in p for p in body["pods"]), "private facts stay out"
    cached = run.status_cache[("shop", "deployment/api")]
    assert cached["pending"] == 2 and cached["fullImages"] == {"api": "registry.io/team/api:1.2.3", "side": "busybox"}
    assert any(p["waitingMessage"] == "rpc error: pull access denied" for p in cached["pods"])
    spec = run.spec_cache[("shop", "deployment/api")]
    assert spec is not None and spec["spec"]["replicas"] == 2
    assert [p["metadata"]["name"] for p in run.pods_cache[("shop", "deployment/api")]] == [
        "api-7d9f8b6c5-ccccc", "api-7d9f8b6c5-ddddd"]
    # No matchLabels: the pods were not listed, which the review must not read as 'no pods'.
    ds_run = Run(namespace="shop", name="shop", app=_app([{"namespace": "shop", "kind": "DaemonSet", "name": "a"}]),
                 excluded=[])
    ds = {"spec": {"selector": {}, "template": {"spec": {"containers": [{"name": "a", "image": "a"}]}}}, "status": {}}
    _invoke(ds_run, "get_workload_status", {"kind": "daemonset", "name": "a"}, Api(deployment=ds))
    assert ds_run.pods_cache[("shop", "daemonset/a")] is None
    assert ds_run.status_cache[("shop", "daemonset/a")]["limits"] == {}


def test_events_cache_keeps_full_message():
    run = _run([{"namespace": "shop", "kind": "Deployment", "name": "api"}])
    events = {"shop": [_event("Pod", "api-7d9f8b6c5-x2k9p", reason="Failed", message=NOT_FOUND)]}
    result = _invoke(run, "get_recent_events", {}, Api(events))
    (seen,) = _content(result)["events"]
    assert len(NOT_FOUND) > 160 and "fullMessage" not in seen and not seen["message"].endswith("not found")
    (cached,) = run.events_cache
    assert cached["fullMessage"] == NOT_FOUND and cached["message"] == seen["message"]
