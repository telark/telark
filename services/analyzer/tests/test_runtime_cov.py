"""Checks for the model runtime state: probes, runtime.changed on every change, pulls and validate.

Ollama is tests/fakes.FakeOllama behind an httpx.MockTransport; the throttle clock
is injected, so no test waits.

Run: python -m pytest tests/test_runtime_cov.py
"""

import asyncio
import os
import sys

import httpx

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import runtime as R  # noqa: E402
from events import Broadcaster  # noqa: E402
from fakes import FakeOllama, dns_error  # noqa: E402

MODEL = "qwen3:4b"


def _drain(sub):
    out = []
    while not sub.queue.empty():
        out.append(sub.queue.get_nowait())
    return out


def _run(fake, scenario, clock=None, mode="deep"):
    """Runs scenario(runtime) on a fresh loop; returns (its result, runtime, published events)."""
    events = Broadcaster()
    sub = events.subscribe(set())

    async def go():
        async with fake.client() as client:
            rt = R.Runtime(client, events, clock=clock, mode=mode) if clock else R.Runtime(client, events, mode=mode)
            result = await scenario(rt)
            if rt.pull_task:
                await rt.pull_task
            return result, rt

    result, rt = asyncio.run(go())
    return result, rt, _drain(sub)


def test_each_transition_publishes_once():
    fake = FakeOllama(models=())

    async def scenario(rt):
        seen = []
        for setup in (
            lambda: fake.overrides.update({"/api/tags": dns_error()}),
            lambda: fake.overrides.update({"/api/tags": httpx.ConnectError("refused")}),
            lambda: fake.overrides.clear(),
            lambda: (fake.models.add(MODEL), fake.capabilities.remove("tools")),
            lambda: fake.capabilities.append("tools"),
        ):
            setup()
            seen.append(await rt.check(MODEL))
            seen.append(await rt.check(MODEL))
        return seen

    seen, rt, events = _run(fake, scenario)

    states = ["absent", "unreachable", "model_missing", "unsupported", "ready"]
    assert seen == [s for s in states for _ in range(2)]
    assert [name for name, _ in events] == ["runtime.changed"] * len(states)
    assert [data["state"] for _, data in events] == states
    assert events[0][1] == {"state": "absent", "model": MODEL, "reason": "ConnectError", "mode": "deep",
                            "autoPull": True, "enabled": False}
    assert events[-1][1] == {"state": "ready", "model": MODEL, "reason": "", "mode": "deep", "autoPull": True,
                             "enabled": False}
    assert rt.status.model == MODEL
    # check never pulls.
    assert fake.count("/api/pull") == 0


def test_show_failure_is_unreachable():
    fake = FakeOllama()
    fake.overrides["/api/show"] = httpx.Response(500, text="boom")

    async def scenario(rt):
        return await rt.check(MODEL), rt.status.reason

    (state, reason), _, _ = _run(fake, scenario)
    assert (state, reason) == ("unreachable", "500")


def test_pull_progress_is_throttled():
    fake = FakeOllama(models=())
    fake.pull_lines = (
        [{"status": "pulling manifest"}]
        + [{"status": "pulling abc", "total": 100, "completed": i} for i in range(1, 6)]
        + [{"status": "verifying digest"}, {"status": "success"}]
    )
    ticks = iter([0.0, 0.1, 0.2, 0.3, 1.2, 1.3, 1.4, 1.5])

    async def scenario(rt):
        await rt.check(MODEL)
        rt.start_pull(MODEL)
        assert rt.pulling and rt.status.state == "pulling"
        # While the pull runs, check reports pulling without probing.
        assert await rt.check(MODEL) == "pulling"
        return None

    _, rt, events = _run(fake, scenario, clock=lambda: next(ticks))

    pulls = [data for name, data in events if name == "runtime.pull"]
    assert [(p["status"], p["completed"]) for p in pulls] == [
        ("pulling manifest", 0), ("pulling abc", 1), ("pulling abc", 4), ("verifying digest", 0), ("success", 0)]
    assert pulls[0]["model"] == MODEL
    changed = [data["state"] for name, data in events if name == "runtime.changed"]
    assert changed == ["model_missing", "pulling", "ready"]
    assert fake.paths.count("/api/pull") == 1 and fake.paths.count("/api/show") == 2
    assert rt.status.pull is None and not rt.pulling


def test_pull_error_sets_reason_until_the_next_pull():
    fake = FakeOllama(models=())
    fake.pull_lines = [{"status": "pulling manifest"}, {"error": "pull model manifest: file does not exist"}]

    async def scenario(rt):
        rt.start_pull(MODEL)
        await rt.pull_task
        after_error = rt.status.model_copy()
        fake.pull_lines = [{"status": "success"}]
        rt.start_pull(MODEL)
        return after_error

    after_error, rt, _ = _run(fake, scenario)
    assert (after_error.state, after_error.reason) == ("model_missing", "pull model manifest: file does not exist")
    assert (rt.status.state, rt.status.reason) == ("ready", "")


def test_pull_failure_logs_once_per_streak(monkeypatch):
    fake = FakeOllama(models=())
    failing = [{"error": "registry unreachable"}]
    warnings = []
    monkeypatch.setattr(R.logger, "warning", lambda *args: warnings.append(args))

    async def scenario(rt):
        for lines in (failing, failing, failing, [{"status": "success"}]):
            fake.pull_lines = lines
            rt.start_pull(MODEL)
            await rt.pull_task
        fake.models.clear()
        fake.pull_lines = failing
        rt.start_pull(MODEL)
        return None

    _run(fake, scenario)
    assert fake.count("/api/pull") == 5
    assert len(warnings) == 2, "one per failure streak: the first three pulls, then the one after the success"


def test_pull_transport_failure_reports_type():
    fake = FakeOllama(models=())
    fake.overrides["/api/pull"] = httpx.RemoteProtocolError("reset")

    async def scenario(rt):
        rt.start_pull(MODEL)
        return None

    _, rt, _ = _run(fake, scenario)
    assert (rt.status.state, rt.status.reason) == ("model_missing", "RemoteProtocolError")


def test_ensure_model_pulls_only_a_missing_model_with_auto_pull(monkeypatch):
    fake = FakeOllama(models=())

    async def scenario(rt):
        rt.ensure_model(MODEL)  # state not known to be model_missing yet
        await rt.check(MODEL)
        monkeypatch.setattr(R, "OLLAMA_AUTO_PULL", False)
        rt.ensure_model(MODEL)
        monkeypatch.setattr(R, "OLLAMA_AUTO_PULL", True)
        rt.ensure_model(MODEL)
        rt.ensure_model(MODEL)  # a pull runs: no second one
        rt.start_pull(MODEL)
        return None

    _, rt, _ = _run(fake, scenario)
    assert fake.count("/api/pull") == 1
    assert rt.status.state == "ready"


def test_pull_started_while_probing_keeps_pulling():
    fake = FakeOllama(models=())
    holder = {}

    def hook(path):
        if path == "/api/show" and "started" not in holder:
            holder["started"] = True
            holder["rt"].start_pull(MODEL)

    fake.hook = hook

    async def scenario(rt):
        holder["rt"] = rt
        return await rt.check(MODEL), rt.status.state

    (state, during), rt, events = _run(fake, scenario)
    assert (state, during) == ("pulling", "pulling")
    assert [d["state"] for n, d in events if n == "runtime.changed"] == ["pulling", "ready"]


def test_validate():
    fake = FakeOllama(models=("qwen3:4b", "gemma3:4b"))

    async def scenario(rt):
        out = {"bad": await rt.validate("Bad Name!")}
        fake.overrides["/api/tags"] = dns_error()
        out["absent"] = await rt.validate(MODEL)
        fake.overrides["/api/tags"] = httpx.ConnectError("refused")
        out["unreachable"] = await rt.validate(MODEL)
        fake.overrides.clear()
        out["missing"] = await rt.validate("qwen2.5:3b")
        fake.capabilities = ["completion", "vision"]
        out["no_tools"] = await rt.validate("gemma3:4b")
        fake.capabilities = ["completion", "tools"]
        out["ok"] = await rt.validate(MODEL)
        return out

    out, rt, events = _run(fake, scenario)

    assert (out["bad"].ok, out["bad"].reason) == (False, "invalid_model_name")
    assert out["absent"].reason == "runtime_absent"
    assert out["unreachable"].reason == "runtime_unreachable"
    missing = out["missing"]
    assert (missing.ok, missing.reason, missing.license) == (False, "model_not_installed", "Qwen Research (non-commercial)")
    assert missing.warning
    no_tools = out["no_tools"]
    assert (no_tools.ok, no_tools.reason, no_tools.license) == (False, "model_lacks_tools", "")
    assert no_tools.capabilities == ["completion", "vision"]
    ok = out["ok"]
    assert (ok.ok, ok.reason, ok.license, ok.warning, ok.model) == (True, "", "Apache-2.0", "", MODEL)
    assert ok.capabilities == ["completion", "tools"]
    # validate reports; it never changes the runtime state.
    assert events == [] and rt.status.state == "unreachable"


# ---- modes ------------------------------------------------------------------------------
def test_fast_mode_ignores_tools_capability():
    fake = FakeOllama(capabilities=("completion",))

    async def scenario(rt):
        return await rt.check(MODEL)

    assert _run(fake, scenario, mode="fast")[0] == "ready"


def test_deep_mode_requires_tools():
    fake = FakeOllama(capabilities=("completion",))

    async def scenario(rt):
        return await rt.check(MODEL)

    assert _run(fake, scenario, mode="deep")[0] == "unsupported"


def test_status_carries_mode_and_autopull(monkeypatch):
    monkeypatch.setattr(R, "OLLAMA_AUTO_PULL", False)

    async def scenario(rt):
        # Before the first check: a GET must already show the mode and the pull switch.
        return rt.status.model_dump()

    before, _, _ = _run(FakeOllama(), scenario, mode="fast")
    assert before == {"state": "unreachable", "model": "", "reason": "", "mode": "fast", "autoPull": False,
                      "enabled": False}


def test_runtime_changed_carries_mode_and_autopull():
    fake = FakeOllama(models=())

    async def scenario(rt):
        await rt.check(MODEL)
        rt.start_pull(MODEL)
        return None

    _, _, events = _run(fake, scenario, mode="fast")
    changed = [data for name, data in events if name == "runtime.changed"]
    assert [d["state"] for d in changed] == ["model_missing", "pulling", "ready"]
    # The UI replaces its runtime object with this payload: without them the fields would vanish.
    assert all((d["mode"], d["autoPull"]) == ("fast", True) for d in changed)


def test_enabled_change_publishes_once():
    async def scenario(rt):
        rt.set_enabled(True)
        rt.set_enabled(True)
        rt.set_enabled(False)
        return rt.status.enabled

    enabled, _, events = _run(FakeOllama(), scenario, mode="fast")
    assert enabled is False
    assert [(name, data["enabled"]) for name, data in events] == [("runtime.changed", True),
                                                                  ("runtime.changed", False)]
    assert events[0][1]["mode"] == "fast"


def test_validate_fast_ok_without_tools():
    fake = FakeOllama(models=("granite4:350m",), capabilities=("completion",))

    async def scenario(rt):
        return await rt.validate("granite4:350m")

    resp, _, _ = _run(fake, scenario, mode="fast")
    assert (resp.ok, resp.reason, resp.license, resp.capabilities) == (True, "", "Apache-2.0", ["completion"])


def test_validate_research_license_warning_kept():
    fake = FakeOllama(models=("qwen2.5:3b",))

    async def scenario(rt):
        return await rt.validate("qwen2.5:3b")

    resp, _, _ = _run(fake, scenario, mode="fast")
    assert (resp.ok, resp.license) == (True, "Qwen Research (non-commercial)") and resp.warning
    for model in ("granite4:350m", "qwen3:1.7b", "qwen3:4b", "qwen2.5:7b"):
        assert R.LICENSES[model] == ("Apache-2.0", "")


def test_undecodable_runtime_is_unreachable():
    # A proxy answering HTML in front of a runtimeUrl: a runtime failure, never an exception.
    fake = FakeOllama()
    fake.overrides["/api/tags"] = httpx.Response(200, text="<html>login</html>")

    async def scenario(rt):
        return await rt.check(MODEL), rt.status.reason

    assert _run(fake, scenario, mode="fast")[0] == ("unreachable", "JSONDecodeError")


def test_ensure_model_pulls_catalog_models_only(monkeypatch):
    monkeypatch.setattr(R, "OLLAMA_AUTO_PULL", True)
    fake = FakeOllama(models=())

    async def scenario(rt):
        for model in ("llama3.1:405b", MODEL):
            await rt.check(model)
            rt.ensure_model(model)
            if rt.pull_task:
                await rt.pull_task

    _run(fake, scenario)
    assert fake.count("/api/pull") == 1
    assert R.pull_allowed(MODEL) and not R.pull_allowed("llama3.1:405b")


def test_pull_past_its_deadline_is_canceled(monkeypatch):
    monkeypatch.setattr(R, "PULL_DEADLINE_S", 0.01)

    async def hang(client, model, on_progress):
        await asyncio.sleep(60)

    monkeypatch.setattr(R.ollama, "pull", hang)
    fake = FakeOllama(models=())

    async def scenario(rt):
        rt.start_pull(MODEL)
        await rt.pull_task
        return rt.status.state, rt.status.reason

    (state, reason), rt, _ = _run(fake, scenario)
    assert (state, reason) == ("model_missing", "TimeoutError") and not rt.pulling
