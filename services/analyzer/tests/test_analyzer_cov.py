"""Checks for the analyzer: the deep loop (fit check, wall reserve, 2-strike abort, EMIT decode, failure map)
and the fast path (gather + rules, one tool-less narration that never fails the run).

Ollama is a scripted httpx.MockTransport that answers /api/chat in order and
records every request; tools.invoke is replaced by canned ToolResults; the clock
is a list and sleep is injected, so no test waits or reaches a model.

Run: python -m pytest tests/test_analyzer_cov.py
"""

import asyncio
import json
import os
import sys
from datetime import UTC, datetime, timedelta
from typing import get_args

import httpx
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import analyzer  # noqa: E402
import insights  # noqa: E402
import messages  # noqa: E402
import tools  # noqa: E402
from models import AppInsights, Candidate, ChatMessage, Emitted, EvidenceRef, Job, Run, ToolResult  # noqa: E402
from prompts.analyzer_prompt import (  # noqa: E402
    EMIT_MESSAGE,
    EMIT_SCHEMA,
    NARRATE_SYSTEM,
    SYSTEM_PROMPT,
    narrate_message,
    narrate_schema,
    user_message,
)
from providers import ollama  # noqa: E402
from tools import k8s_tools  # noqa: E402
from tools.k8s_tools import K8s  # noqa: E402

JOB = Job(namespace="shop", name="shop", trigger="incident", generation=7)

APP = {
    "name": "shop",
    "namespaces": {"items": [{"name": "shop"}]},
    "resources": [{"namespace": "shop", "kind": "Deployment", "name": "api"}],
    "history": {"generation": 7, "changeLog": [
        {"generation": 6, "changeClass": "config", "severity": "low"},
        {"generation": 7, "changeClass": "image", "severity": "high", "isIncident": True},
    ]},
}

INSIGHT = {
    "kind": "crashloop", "subject": "deployment/api", "title": "api crash-loops",
    "summary": "The new image exits at start.", "confidence": "high", "severity": "critical",
    "evidence": [{"type": "workload", "ref": "workload:deployment/api"}, {"type": "change", "ref": "gen:7"}],
}

WORKLOAD = ("get_workload_status", {"kind": "deployment", "name": "api"})
OVERVIEW = ("get_app_overview", {})
OVERFLOW = (400, '{"error":"input length exceeds the context length"}')
BUSY = (503, "server busy")


def _chat(content="", calls=(), prompt_eval=100, evals=20):
    message = {"role": "assistant", "content": content}
    if calls:
        message["tool_calls"] = [{"function": {"name": n, "arguments": a}} for n, a in calls]
    return 200, {"model": "m", "message": message, "done": True,
                 "prompt_eval_count": prompt_eval, "eval_count": evals}


def _emit(*insights):
    return _chat(json.dumps({"insights": list(insights)}))


class Ollama:
    """Answers /api/chat from a script, in order, and records every request."""

    def __init__(self, *script):
        self.script = list(script)
        self.bodies: list[dict] = []
        self.timeouts: list[float] = []

    def __call__(self, request):
        assert request.url.path == "/api/chat"
        self.bodies.append(json.loads(request.content))
        self.timeouts.append(request.extensions["timeout"]["read"])
        step = self.script.pop(0)
        if isinstance(step, Exception):
            raise step
        status, body = step
        return httpx.Response(status, json=body) if isinstance(body, dict) else httpx.Response(status, text=body)

    def emits(self):
        return [b for b in self.bodies if "format" in b]


class Tools:
    """Canned tools.invoke: one error code per call ('' = ok), mirroring the real counters and flag."""

    def __init__(self, *errors):
        self.errors = list(errors)
        self.calls: list[tuple[str, dict]] = []

    async def __call__(self, run, name, arguments, k8s):
        self.calls.append((name, arguments))
        error = self.errors.pop(0) if self.errors else ""
        if error in ("", "timeout", "k8s_error"):
            run.tool_calls += 1
        if error in ("timeout", "k8s_error", "k8s_unavailable"):
            run.truncated = True
        content = json.dumps({"error": error, "detail": ""}) if error else '{"ok":true}'
        return ToolResult(name=name, content=content, error=error)


def _analyze(monkeypatch, script, stub=None, clock=(0,), slept=None):
    monkeypatch.setattr(tools, "invoke", stub or Tools())
    ticks = list(clock)
    slept = [] if slept is None else slept

    def fake_clock():
        return ticks.pop(0) if len(ticks) > 1 else ticks[0]

    async def fake_sleep(seconds):
        slept.append(seconds)

    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(script), base_url="http://ollama") as client:
            run = Run(namespace="shop", name="shop", app=APP, excluded=[])
            return await analyzer.run_analysis(JOB, run, client, None, "qwen3:4b", clock=fake_clock, sleep=fake_sleep)

    outcome = asyncio.run(go())
    _assert_protocol(script)
    return outcome


def _assert_protocol(script):
    """Every request: no thinking, no silent truncate/shift, the same tools; only EMIT carries format."""
    for body in script.bodies:
        assert (body["think"], body["truncate"], body["shift"], body["stream"]) == (False, False, False, False)
        assert body["tools"] == tools.SPECS
        is_emit = any(m["role"] == "user" and m["content"] == EMIT_MESSAGE for m in body["messages"])
        assert ("format" in body) is is_emit
        assert body["options"]["num_predict"] == (1024 if is_emit else 400)
        if is_emit:
            assert body["format"] == EMIT_SCHEMA


def test_loop_happy_path(monkeypatch):
    stub = Tools()
    script = Ollama(_chat(calls=[OVERVIEW]), _chat(calls=[WORKLOAD]), _chat("The api crash-loops."),
                    _emit(INSIGHT, {**INSIGHT, "kind": "other", "subject": "deployment/web", "evidence": []}))
    outcome = _analyze(monkeypatch, script, stub)

    assert outcome.error == ""
    assert [e.subject for e in outcome.emitted] == ["deployment/api", "deployment/web"]
    assert (outcome.steps, outcome.tool_calls, outcome.truncated) == (3, 2, False)
    assert stub.calls == [OVERVIEW, WORKLOAD]
    assert script.timeouts == [120, 120, 120, 180]
    emit = script.emits()
    assert len(emit) == 1
    messages = emit[0]["messages"]
    assert [m["role"] for m in messages] == [
        "system", "user", "assistant", "tool", "assistant", "tool", "assistant", "user"]
    assert [m.get("tool_name") for m in messages if m["role"] == "tool"] == ["get_app_overview", "get_workload_status"]
    assert messages[0]["content"] == SYSTEM_PROMPT
    assert messages[-1]["content"] == EMIT_MESSAGE


def test_two_strikes_abort(monkeypatch):
    stub = Tools("unknown_tool", "", "invalid_arguments", "unknown_workload")
    script = Ollama(
        _chat(calls=[("kubectl_delete", {})]),
        _chat(calls=[OVERVIEW]),
        _chat(calls=[("get_workload_status", {"kind": "pod", "name": "x"}),
                     ("get_workload_status", {"kind": "deployment", "name": "ghost"})]),
    )
    outcome = _analyze(monkeypatch, script, stub)

    # The valid overview call between the first and second strike resets the count.
    assert outcome.error == "invalid_tool_calls"
    assert (outcome.emitted, outcome.steps, outcome.tool_calls) == ([], 3, 1)
    assert len(stub.calls) == 4
    assert script.emits() == []


def test_overflow_at_loop_forces_emit(monkeypatch):
    script = Ollama(_chat(calls=[OVERVIEW]), OVERFLOW, _emit(INSIGHT))
    outcome = _analyze(monkeypatch, script)

    assert outcome.error == ""
    assert outcome.truncated is True
    assert [e.subject for e in outcome.emitted] == ["deployment/api"]
    assert outcome.steps == 1
    assert len(script.bodies) == 3 and len(script.emits()) == 1


def test_overflow_at_emit_fails(monkeypatch):
    script = Ollama(_chat("Nothing else to check."), OVERFLOW)
    outcome = _analyze(monkeypatch, script)
    assert (outcome.error, outcome.emitted, outcome.steps) == ("context_overflow", [], 1)

    # An EMIT that cannot fit is refused in code, before any request.
    base = [ChatMessage(role="system", content=SYSTEM_PROMPT), ChatMessage(role="user", content=user_message(JOB, APP))]
    est = analyzer.estimate([*base, ChatMessage(role="user", content=EMIT_MESSAGE)], 0, 0)
    monkeypatch.setattr(analyzer, "ANALYZER_CONTEXT_TOKENS", int(est) + analyzer.NUM_PREDICT_EMIT - 1)
    script = Ollama()
    outcome = _analyze(monkeypatch, script)
    assert (outcome.error, outcome.steps, script.bodies) == ("context_overflow", 0, [])


def test_fit_check_is_per_request():
    small = [ChatMessage(role="user", content="u")]
    # The last counters describe the whole request already: never a sum across calls.
    assert analyzer.estimate(small, 5000, 300) == 5300
    big = [ChatMessage(role="tool", content="x" * 7000, tool_name="t")]
    size = len(json.dumps([m.model_dump() for m in big]).encode())
    assert analyzer.estimate(big, 10, 10) == size / 3.5

    ctx = analyzer.ANALYZER_CONTEXT_TOKENS
    loop_max = ctx - 735 - 100 - 1024
    assert analyzer.fits_loop(loop_max) and not analyzer.fits_loop(loop_max + 1)
    assert analyzer.fits_emit(ctx - 1024) and not analyzer.fits_emit(ctx - 1023)


def test_emit_reserve(monkeypatch):
    # 400 s into a 480 s wall: 80 s left < 300 s reserve, so no loop step is taken.
    script = Ollama(_emit(INSIGHT))
    outcome = _analyze(monkeypatch, script, clock=(0, 400))

    assert outcome.error == ""
    assert (outcome.steps, outcome.truncated, len(outcome.emitted)) == (0, True, 1)
    assert len(script.bodies) == 1 and "format" in script.bodies[0]
    assert script.timeouts == [80]


def test_emit_decode_retry(monkeypatch):
    bad = json.dumps({"insights": [{**INSIGHT, "kind": "meltdown"}]})
    script = Ollama(_chat("done"), _chat(bad), _emit(INSIGHT))
    outcome = _analyze(monkeypatch, script)

    assert outcome.error == ""
    assert [e.kind for e in outcome.emitted] == ["crashloop"]
    retry = script.emits()[1]["messages"]
    assert retry[-2]["content"] == EMIT_MESSAGE
    assert retry[-1]["role"] == "user"
    assert "literal_error:insights.0.kind" in retry[-1]["content"]
    # Error type and location only: the raw completion never goes back to the model.
    assert "meltdown" not in json.dumps(retry)

    too_many = json.dumps({"insights": [INSIGHT] * 4})
    script = Ollama(_chat("done"), _chat("not json"), _chat(too_many))
    outcome = _analyze(monkeypatch, script)
    assert (outcome.error, outcome.emitted) == ("invalid_output", [])
    assert len(script.emits()) == 2


def test_busy_retry_once(monkeypatch):
    slept = []
    outcome = _analyze(monkeypatch, Ollama(BUSY, _chat("done"), _emit()), slept=slept)
    assert (outcome.error, outcome.emitted, outcome.steps) == ("", [], 1)
    assert slept == [15]

    slept = []
    script = Ollama(BUSY, BUSY)
    outcome = _analyze(monkeypatch, script, slept=slept)
    assert outcome.error == "busy"
    assert slept == [15] and len(script.bodies) == 2


def test_tool_timeout_truncates(monkeypatch):
    stub = Tools("timeout", "k8s_error")
    script = Ollama(_chat(calls=[WORKLOAD]), _chat(calls=[("get_recent_events", {})]), _chat("done"), _emit(INSIGHT))
    outcome = _analyze(monkeypatch, script, stub)

    # Not strikes, and the model chose to stop (3 loop steps): truncated comes from the run, not a forced EMIT.
    assert outcome.error == ""
    assert (outcome.truncated, outcome.steps, outcome.tool_calls) == (True, 3, 2)
    assert len(script.bodies) == 4


@pytest.mark.parametrize(
    ("step", "code"),
    [
        (httpx.ReadTimeout("slow"), "model_timeout"),
        (httpx.RemoteProtocolError("reset"), "runtime_unreachable"),
        ((400, '{"error":"registry.ollama.ai/library/x does not support tools"}'), "model_unsupported"),
        ((404, '{"error":"model not found"}'), "model_not_installed"),
    ],
)
def test_runtime_failures_map_to_codes(monkeypatch, step, code):
    assert _analyze(monkeypatch, Ollama(step)).error == code


def test_emit_timeout_and_unknown_error(monkeypatch):
    outcome = _analyze(monkeypatch, Ollama(_chat("done"), httpx.ReadTimeout("slow")))
    assert (outcome.error, outcome.steps) == ("model_timeout", 1)
    # An unmapped runtime error is not a lastRun code: the worker's catch-all records it.
    with pytest.raises(ollama.OllamaError):
        _analyze(monkeypatch, Ollama((500, '{"error":"boom"}')))


def test_step_limits(monkeypatch):
    # At most two tool calls per step; the extra one is dropped from the history too.
    stub = Tools()
    script = Ollama(_chat(calls=[OVERVIEW, WORKLOAD, ("get_recent_events", {})]), _chat("done"), _emit())
    _analyze(monkeypatch, script, stub)
    assert stub.calls == [OVERVIEW, WORKLOAD]
    assert len(script.bodies[1]["messages"][2]["tool_calls"]) == 2

    # No tool_calls but a {name, arguments} object in content: parsed once and invoked.
    stub = Tools()
    script = Ollama(_chat(json.dumps({"name": "get_app_overview", "arguments": {}})), _chat("done"), _emit())
    outcome = _analyze(monkeypatch, script, stub)
    assert stub.calls == [OVERVIEW] and outcome.steps == 2

    # A near-full context forces the truncated EMIT before the next step.
    script = Ollama(_chat(calls=[OVERVIEW], prompt_eval=analyzer.ANALYZER_CONTEXT_TOKENS - 1300), _emit())
    outcome = _analyze(monkeypatch, script)
    assert (outcome.steps, outcome.truncated, len(script.bodies)) == (1, True, 2)

    monkeypatch.setattr(analyzer, "ANALYZER_MAX_STEPS", 1)
    outcome = _analyze(monkeypatch, Ollama(_chat(calls=[OVERVIEW]), _emit()))
    assert (outcome.steps, outcome.truncated) == (1, True)

    monkeypatch.setattr(analyzer, "ANALYZER_MAX_STEPS", 8)
    monkeypatch.setattr(analyzer, "ANALYZER_MAX_TOOL_CALLS", 2)
    outcome = _analyze(monkeypatch, Ollama(_chat(calls=[OVERVIEW, WORKLOAD]), _emit()))
    assert (outcome.steps, outcome.tool_calls, outcome.truncated) == (1, 2, True)


def test_prompt_contract():
    assert json.loads(user_message(JOB, APP)) == {
        "namespace": "shop", "name": "shop", "trigger": "incident",
        "lastChange": {"generation": 7, "changeClass": "image", "severity": "high"},
    }
    assert json.loads(user_message(JOB, {}))["lastChange"] is None

    # The schema the runtime enforces and the model the analyzer decodes into must agree.
    insights = EMIT_SCHEMA["properties"]["insights"]
    assert insights["maxItems"] == 3
    props = insights["items"]["properties"]
    for name in ("kind", "confidence", "severity"):
        assert set(props[name]["enum"]) == set(get_args(Emitted.model_fields[name].annotation))
    # reason/params are set by the rules only: the deep EMIT schema never asks the model for them.
    assert set(insights["items"]["required"]) == set(Emitted.model_fields) - {"reason", "params"}
    assert props["evidence"]["maxItems"] == 4
    assert props["evidence"]["items"]["properties"]["type"]["enum"] == ["change", "snapshot", "event", "workload"]


# ---- fast path: gather + rules ------------------------------------------------------------
NOW = datetime(2026, 9, 24, 12, 0, tzinfo=UTC)
POD = "api-7d9f8b6c5-x2k4q"
FAST_APP = {
    "name": "shop",
    "health": {"status": "degraded", "readyReplicas": 1, "totalReplicas": 3},
    "namespaces": {"items": [{"name": "shop"}]},
    "resources": [{"namespace": "shop", "kind": "Deployment", "name": w} for w in ("web", "api")],
    "history": {"generation": 7, "changeLog": [
        {"generation": 7, "detectedAt": "2026-09-24T11:50:00Z", "changeClass": "image", "severity": "high",
         "isIncident": True, "changes": [{"field": "image", "oldValue": "api:1", "newValue": "api:2"}]}]},
}


class Kube:
    """A fake kube-apiserver: one crash-looping api pod with its BackOff event; web is healthy."""

    def __init__(self):
        self.paths: list[str] = []

    def __call__(self, request):
        path = request.url.path
        self.paths.append(path)
        if path.endswith("/events"):
            # lastTimestamp sits on the real clock (the events window); firstTimestamp on the run's (the correlation).
            last = (datetime.now(UTC) - timedelta(minutes=2)).strftime("%Y-%m-%dT%H:%M:%SZ")
            return httpx.Response(200, json={"items": [{
                "involvedObject": {"kind": "Pod", "name": POD}, "reason": "BackOff", "count": 7,
                "message": "Back-off restarting failed container api", "lastTimestamp": last,
                "firstTimestamp": NOW.strftime("%Y-%m-%dT%H:%M:%SZ")}]})
        name = path.rsplit("/", 1)[-1]
        if path.endswith("/pods"):
            pods = [{"metadata": {"name": POD}, "status": {"phase": "Running", "containerStatuses": [{
                "restartCount": 7, "state": {"waiting": {"reason": "CrashLoopBackOff"}},
                "lastState": {"terminated": {"reason": "Error", "exitCode": 1}}}]}}]
            return httpx.Response(200, json={"items": pods if request.url.params["labelSelector"] == "app=api" else []})
        ready = 0 if name == "api" else 2
        return httpx.Response(200, json={
            "spec": {"replicas": 2 if name == "web" else 1, "selector": {"matchLabels": {"app": name}},
                     "template": {"spec": {"containers": [{"image": f"reg/{name}:2"}]}}},
            "status": {"updatedReplicas": 2 if name == "web" else 1, "readyReplicas": ready}})


def _gather(monkeypatch, tmp_path, app=FAST_APP, kube=None):
    token = tmp_path / "token"
    token.write_text("t")
    monkeypatch.setattr(k8s_tools, "SA_TOKEN_PATH", str(token))
    timings: dict = {}

    async def go():
        run = Run(namespace="shop", name="shop", app=app, excluded=[])
        if kube is None:
            return run, await analyzer.gather_rules(run, None, NOW, timings)
        async with httpx.AsyncClient(transport=httpx.MockTransport(kube), base_url="https://k8s") as client:
            return run, await analyzer.gather_rules(run, K8s(client), NOW, timings)

    run, candidates = asyncio.run(go())
    return run, candidates, timings


def test_gather_rules_happy_path(monkeypatch, tmp_path):
    kube = Kube()
    run, candidates, timings = _gather(monkeypatch, tmp_path, kube=kube)

    (c,) = candidates
    assert (c.subject, c.kind, c.severity, c.confidence) == ("deployment/api", "crashloop", "critical", "high")
    refs = [e.ref for e in c.evidence]
    assert refs[:2] == ["workload:deployment/api", "gen:7"] and refs[2].startswith(f"BackOff@Pod/{POD}@")
    assert "10 min after change gen 7: image api:1\u2192api:2" in c.facts
    # overview + history + events + the two workloads' status (event-named api first): within 4 + 3.
    assert run.calls == {"get_app_overview": 1, "get_change_history": 1, "get_recent_events": 1,
                         "get_workload_status": 2}
    assert run.tool_calls == 5 and run.truncated is False
    assert [p.rsplit("/", 1)[-1] for p in kube.paths if "/apis/apps/" in p] == ["api", "web"]
    assert set(run.status_cache) == {("shop", "deployment/api"), ("shop", "deployment/web")}
    assert set(timings) == {"gather", "rules"}
    emitted = c.to_emitted()
    assert isinstance(emitted, Emitted) and (emitted.kind, emitted.title) == ("crashloop", c.title)


def test_gather_rules_k8s_unavailable_truncated(monkeypatch, tmp_path):
    app = {**FAST_APP, "resources": FAST_APP["resources"][1:]}
    run, candidates, _ = _gather(monkeypatch, tmp_path, app=app)
    # Overview + history only; the lone workload of a degraded app is the one the change hit.
    assert run.truncated is True and run.tool_calls == 2
    assert [(c.kind, c.confidence) for c in candidates] == [("config_change_regression", "medium")]


def test_gather_rules_no_workloads_no_candidates(monkeypatch, tmp_path):
    kube = Kube()
    run, candidates, _ = _gather(monkeypatch, tmp_path, app={**FAST_APP, "resources": []}, kube=kube)
    assert candidates == [] and run.calls["get_workload_status"] == 0
    assert not any("/apis/apps/" in p for p in kube.paths)


TWIN_POD = "web-7d9f8b6c5-x2k4q"
TWIN_APP = {
    "name": "cart",
    "health": {"status": "degraded", "readyReplicas": 1, "totalReplicas": 2},
    "namespaces": {"items": [{"name": "shop-dev"}, {"name": "shop-prod"}]},
    "resources": [{"namespace": ns, "kind": "Deployment", "name": "web"} for ns in ("shop-dev", "shop-prod")],
    "history": {"generation": 7, "changeLog": []},
}


class TwinKube:
    """Deployment web in two namespaces, with the same pod name in both; the `broken` ones crash-loop."""

    def __init__(self, broken):
        self.broken = broken

    def __call__(self, request):
        path = request.url.path
        namespace = path.split("/namespaces/")[1].split("/")[0]
        broken = namespace in self.broken
        if path.endswith("/events"):
            last = (datetime.now(UTC) - timedelta(minutes=2)).strftime("%Y-%m-%dT%H:%M:%SZ")
            items = [{"involvedObject": {"kind": "Pod", "name": TWIN_POD}, "reason": "BackOff", "count": 7,
                      "message": "Back-off restarting failed container web", "lastTimestamp": last}] if broken else []
            return httpx.Response(200, json={"items": items})
        if path.endswith("/pods"):
            state = {"waiting": {"reason": "CrashLoopBackOff"}} if broken else {"running": {}}
            return httpx.Response(200, json={"items": [{"metadata": {"name": TWIN_POD}, "status": {
                "phase": "Running", "containerStatuses": [{"restartCount": 7 if broken else 0, "state": state}]}}]})
        return httpx.Response(200, json={
            "spec": {"replicas": 1, "selector": {"matchLabels": {"app": "web"}},
                     "template": {"spec": {"containers": [{"image": "reg/web:2"}]}}},
            "status": {"updatedReplicas": 1, "readyReplicas": 0 if broken else 1}})


def _twin_cards(monkeypatch, tmp_path, broken):
    run, candidates, _ = _gather(monkeypatch, tmp_path, app=TWIN_APP, kube=TwinKube(broken))
    doc = AppInsights()
    insights.merge(doc, insights.validate([c.to_emitted() for c in candidates], run), NOW.isoformat(), False,
                   "shop-dev", "cart")
    return doc.insights


@pytest.mark.parametrize("broken", ["shop-dev", "shop-prod"])
def test_same_workload_in_two_namespaces_one_broken(monkeypatch, tmp_path, broken):
    (card,) = _twin_cards(monkeypatch, tmp_path, {broken})
    assert (card.subject, card.kind, card.params["namespace"]) == ("deployment/web", "crashloop", broken)
    assert card.id == insights.insight_id("shop-dev", "cart", "deployment/web", broken)


def test_same_workload_in_two_namespaces_both_broken(monkeypatch, tmp_path):
    cards = _twin_cards(monkeypatch, tmp_path, {"shop-dev", "shop-prod"})
    assert sorted(c.params["namespace"] for c in cards) == ["shop-dev", "shop-prod"]
    assert len({c.id for c in cards}) == 2 and {c.kind for c in cards} == {"crashloop"}


# ---- fast path: narration -----------------------------------------------------------------
def _candidate(subject="deployment/api"):
    return Candidate(subject=subject, kind="crashloop", severity="critical", confidence="high",
                     evidence=[EvidenceRef(type="workload", ref="workload:" + subject)], title="t", summary="s",
                     facts=["pod: api-1", "restarts: 5"])


def _narration(*items):
    return _chat(json.dumps({"insights": [{"title": t, "summary": s} for t, s in items]}))


def _narrate(script, candidates=None, slept=None):
    slept = [] if slept is None else slept

    async def fake_sleep(seconds):
        slept.append(seconds)

    async def go():
        async with httpx.AsyncClient(transport=httpx.MockTransport(script), base_url="http://ollama") as client:
            return await analyzer.narrate(client, "granite4:350m", candidates or [_candidate()], sleep=fake_sleep)

    return asyncio.run(go())


def test_narrate_happy_path():
    script = Ollama(_narration(("api crash-loops", "The api pod restarted 5 times.")))
    items, reason = _narrate(script)

    assert reason == "" and [(i.title, i.summary) for i in items] == [("api crash-loops", "The api pod restarted 5 times.")]
    (body,) = script.bodies
    assert "tools" not in body
    assert body["format"] == narrate_schema(1) and body["format"]["properties"]["insights"]["minItems"] == 1
    assert body["options"]["num_thread"] == 2 and body["options"]["num_predict"] == 136
    assert body["keep_alive"] == -1 and body["think"] is False
    assert [m["content"] for m in body["messages"]] == [NARRATE_SYSTEM, narrate_message([_candidate()])]
    assert script.timeouts == [45]


def test_narrate_timeout_returns_none():
    assert _narrate(Ollama(httpx.ReadTimeout("slow"))) == (None, "OllamaTimeout")


def test_narrate_generic_ollama_error_returns_none():
    # A 500 with an unclassified body (a memory refusal) is the OllamaError base: never raised.
    script = Ollama((500, '{"error":"model requires more system memory (3.1 GiB) than is available"}'))
    assert _narrate(script) == (None, "OllamaError")


def test_narrate_busy_retried_once():
    slept = []
    items, _ = _narrate(Ollama(BUSY, _narration(("api t", "s"))), slept=slept)
    assert len(items) == 1 and slept == [15]
    assert _narrate(Ollama(BUSY, BUSY)) == (None, "OllamaBusy")


def test_narrate_wrong_item_count_returns_none():
    script = Ollama(_narration(("a", "b"), ("c", "d")))
    assert _narrate(script) == (None, "item_count")


def test_narrate_undecodable_returns_none():
    assert _narrate(Ollama(_chat("not json"))) == (None, "ValidationError")
    too_long = json.dumps({"insights": [{"title": "t" * 81, "summary": "s"}]})
    assert _narrate(Ollama(_chat(too_long))) == (None, "ValidationError")
    # A non-JSON body is an OllamaError from the client.
    assert _narrate(Ollama((200, "<html>proxy error</html>"))) == (None, "OllamaError")


def _stated(subject="deployment/api"):
    """A candidate whose template states the pod, restarts and image: the narration must keep all three."""
    return Candidate(subject=subject, kind="image_pull", severity="critical", confidence="high", evidence=[],
                     title="api cannot pull its image", summary="Pod api-7d9f-x2: 4 restarts, image nginx:1.99.",
                     facts=["pod: api-7d9f-x2", "restarts: 4", "image: nginx:1.99", "message: not in the template"])


def test_narrate_keeps_only_faithful_items():
    faithful = ("api cannot pull nginx:1.99", "Pod api-7d9f-x2 fails to pull image nginx:1.99 after 4 restarts.")
    unfaithful = [
        ("Image pull failure", "Pod api-7d9f-x2 fails to pull image nginx:1.99 after 4 restarts."),  # no name in title
        ("api cannot pull its image", "Pod api-7d9f-x2 fails after 4 restarts."),  # drops the image
        ("api cannot pull nginx:1.99", "Pod api-7d9f-x2, image nginx:1.99, 4 restarts.}}] api"),  # markup
        ("api cannot pull nginx:1.99", "Pod api-66f-q9 and api-7d9f-x2, image nginx:1.99, 4 restarts."),  # new name
        ("Api cannot pull nginx:1.99", "Pod api-7d9f-x2, image nginx:1.99, 4 restarts."),  # name not exact
        ("api cannot pull nginx:1.99", ("Pod api-7d9f-x2, image nginx:1.99, 4 restarts. " * 6)[:240]),  # cut at cap
        ("api " + "x" * 76, "Pod api-7d9f-x2, image nginx:1.99, 4 restarts."),  # title cut at cap
        ("api cannot pull nginx:1.99", "Pod api-7d9f-x2, image nginx:1.99, 4 restarts, exit code 137."),  # new number
        ("api cannot pull nginx:1.99", "Pod api-7d9f-x2, image nginx:1.99, 14 restarts."),  # changed number
    ]
    for bad in unfaithful:
        items, reason = _narrate(Ollama(_narration(faithful, bad)), candidates=[_stated(), _stated()])
        assert reason == "" and [i.title for i in items] == [faithful[0], ""], bad
    # Every item unfaithful: nothing to apply, the reason says why.
    assert _narrate(Ollama(_narration(unfaithful[0])), candidates=[_stated()]) == (None, "unfaithful")
    # A value only the facts hold (not the template) may be dropped; a trailing period is not a new name.
    items, _ = _narrate(Ollama(_narration(("api cannot pull its image", "Pod api-7d9f-x2 (4 restarts) cannot "
                                                                         "pull nginx:1.99."))), candidates=[_stated()])
    assert items[0].title == "api cannot pull its image"


def test_narrate_prompt_contract():
    two = [_candidate(), _candidate("deployment/web")]
    assert json.loads(narrate_message(two)) == [
        {"i": 0, "kind": "crashloop", "name": "api", "title": "t", "summary": "s", "what": "", "params": {}},
        {"i": 1, "kind": "crashloop", "name": "web", "title": "t", "summary": "s", "what": "", "params": {}},
    ]
    # The sub-reason's phrase and the params go to the model, never the free facts list.
    (sent,) = json.loads(narrate_message([_pull_candidate()]))
    assert sent["what"] == "image not found" and sent["params"]["image"] == "nginx:1.99" and "facts" not in sent
    assert "Keep the phrase given as what and every number exactly." in NARRATE_SYSTEM
    schema = narrate_schema(2)
    items = schema["properties"]["insights"]
    assert (items["minItems"], items["maxItems"]) == (2, 2)
    assert items["items"]["properties"] == {"title": {"type": "string", "maxLength": 80},
                                            "summary": {"type": "string", "maxLength": 240}}
    assert schema["additionalProperties"] is False and items["items"]["additionalProperties"] is False
    # Byte-stable and first, so Ollama serves the prefix from its cache.
    script = Ollama(_narration(("a", "b"), ("c", "d")))
    _narrate(script, two)
    assert script.bodies[0]["messages"][0]["content"] == NARRATE_SYSTEM
    assert script.bodies[0]["options"]["num_predict"] == 16 + 120 * 2


# ---- narration fact check: numbers, the sub-reason phrase, other kinds' symptoms (S5) --------------------------
def _pull_candidate():
    params = {"workload": "api", "namespace": "shop", "pod": "api-7d9f-x2", "container": "api", "image": "nginx:1.99",
              "registry": "docker.io", "ready": "0", "desired": "2", "message": "nginx:1.99: not found"}
    title, summary = messages.render_incident("image_pull.not_found", params, "api", {"ready": 0, "desired": 2}, "")
    facts = [f"what: {messages.what('image_pull.not_found', params)}", *(f"{k}: {v}" for k, v in params.items())]
    return Candidate(subject="deployment/api", kind="image_pull", severity="critical", confidence="high", evidence=[],
                     title=title, summary=summary, facts=facts, reason="image_pull.not_found", params=params)


FAITHFUL_PULL = ("api is down: image not found",
                 "No replica of api is ready (0 of 2). Pod api-7d9f-x2 cannot pull nginx:1.99: image not found.")


def _judge(item):
    items, reason = _narrate(Ollama(_narration(FAITHFUL_PULL, item)), candidates=[_pull_candidate()] * 2)
    assert reason == "" and items[0].title == FAITHFUL_PULL[0]
    return bool(items[1].title)


def test_narrate_accepts_faithful_rewrite():
    assert _judge(FAITHFUL_PULL)
    # Numbers of the template's own text (not only the facts) are known: '0 of 2' is stated by the impact.
    assert _judge(("api is down: image not found", "0 of 2 replicas are ready. Pod api-7d9f-x2 cannot pull "
                                                   "nginx:1.99: image not found in the registry."))
    # 'room' is not 'oom': words match at a word start only.
    assert _judge(("api is down: image not found", "Pod api-7d9f-x2 cannot pull nginx:1.99 (0 of 2 ready): image "
                                                   "not found, no room to start."))


def test_narrate_rejects_new_number():
    assert not _judge(("api is down: image not found", "Pod api-7d9f-x2 cannot pull nginx:1.99 after 3 attempts: "
                                                       "image not found."))


def test_narrate_rejects_missing_what_phrase():
    assert not _judge(("api is down: pull failed", "Pod api-7d9f-x2 cannot pull nginx:1.99; 0 of 2 replicas ready."))


def test_narrate_rejects_other_kind_word():
    for other in ("it ran out of memory", "it is crash-looping", "it was evicted", "its readiness probe fails",
                  "the rollout is stuck", "pods are pending", "the scheduler cannot place it", "OOM"):
        assert not _judge(("api is down: image not found",
                           f"Pod api-7d9f-x2 cannot pull nginx:1.99, image not found; {other}.")), other
