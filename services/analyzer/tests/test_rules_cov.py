"""Checks for rules.py: every insight kind from minimal tool payloads, precedence, change correlation.

The rules are the fast path's detector: a miss here is an insight of the wrong
kind on every cluster. Inputs are shaped exactly like the tools' results
(run.status_cache, run.events_cache, the overview and history payloads).

Run: python -m pytest tests/test_rules_cov.py
"""

import os
import sys
from datetime import UTC, datetime

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import rules  # noqa: E402
from constants import (  # noqa: E402
    MAX_INSIGHT_SUMMARY_LENGTH,
    MAX_INSIGHT_TITLE_LENGTH,
    REF_EVENT,
)
from models import Run  # noqa: E402

NOW = datetime(2026, 9, 24, 12, 0, tzinfo=UTC)
LAST = "2026-09-24T11:58:00Z"
POD = "api-7d9f8b6c5-x2k4q"
WORKLOAD = "workload:deployment/api"
HEALTHY = {"health": {"status": "healthy", "ready": 2, "total": 2}}
DEGRADED = {"health": {"status": "degraded", "ready": 1, "total": 2}}


def _run(*workloads):
    app = {"resources": [{"namespace": "shop", "kind": "Deployment", "name": w} for w in workloads or ("api",)]}
    run = Run("shop", "shop", app, [])
    run.refs.update(f"workload:{s}" for _ns, s in run.workloads)
    return run


def _status(desired=2, ready=2, updated=None, pods=(), conditions=(), name="api"):
    return {"kind": "deployment", "name": name, "conditions": list(conditions), "desired": desired,
            "updated": desired if updated is None else updated, "ready": ready, "pods": list(pods),
            "images": [f"{name}:1.2.3"]}


def _pod(name=POD, phase="Running", restarts=0, waiting=None, last=None, code=None, container="api"):
    return {"name": name, "phase": phase, "restarts": restarts, "waitingReason": waiting,
            "lastTerminated": last and {"reason": last, "exitCode": code, "at": LAST}, "container": container,
            "init": False, "node": "n1"}


def _event(reason, message="", obj=f"Pod/{POD}", count=1):
    return {"reason": reason, "object": obj, "message": message, "count": count, "first": LAST, "last": LAST,
            "namespace": "shop"}


def _ref(event):
    return REF_EVENT.format(**event)


def _change(minutes_ago=10, generation=42, incident=True, change_class="image", changes=None):
    at = datetime.fromtimestamp(NOW.timestamp() - minutes_ago * 60, UTC).strftime("%Y-%m-%dT%H:%M:%SZ")
    if changes is None:
        changes = [{"field": "image", "changeType": "updated", "oldValue": "api:1", "newValue": "api:2"}]
    return {"generation": generation, "detectedAt": at, "changeClass": change_class, "severity": "high",
            "isIncident": incident, "isRecovery": False, "changes": changes}


def _eval(statuses=None, events=(), overview=HEALTHY, changes=(), run=None):
    """Fills the run the way the tools do (caches + refs), then evaluates."""
    run = run or _run()
    run.status_cache.update({("shop", f"deployment/{s['name']}"): s for s in statuses or ()})
    run.events_cache.extend(events)
    run.refs.update(_ref(e) for e in events)
    run.refs.update(f"gen:{c['generation']}" for c in changes)
    return rules.evaluate(run, overview, {"changes": list(changes)}, run.status_cache, run.events_cache, NOW)


def _one(*args, **kwargs):
    (candidate,) = _eval(*args, **kwargs)
    return candidate


def _shape(c):
    return c.subject, c.kind, c.severity, c.confidence, [e.ref for e in c.evidence]


# ---- one fixture per kind (the sub-reason texts are covered in test_messages_cov) ------------
def test_oom():
    status = _status(ready=1, pods=[_pod(restarts=4, last="OOMKilled", code=137)])
    status["limits"] = {"api": "64Mi"}
    c = _one([status])
    assert _shape(c) == ("deployment/api", "oom", "critical", "high", [WORKLOAD])
    assert [e.type for e in c.evidence] == ["workload"]
    assert (c.reason, c.title) == ("oom.limit", "api is degraded: out of memory")
    assert c.summary == (f"1 of 2 replicas are ready. Container api in pod {POD} used more than its 64Mi memory "
                         "limit and was killed (OOMKilled), 4 restarts.")
    assert "restarts: 4" in c.facts and "limit: 64Mi" in c.facts and c.facts[0] == "what: out of memory"
    assert c.params["workload"] == "api" and c.params["namespace"] == "shop"

    # A kernel OOM event alone is enough; nothing unread is ever stated.
    oom = _event("OOMKilling", "Memory cgroup out of memory")
    c = _one(events=[oom])
    assert _shape(c) == ("deployment/api", "oom", "critical", "high", [WORKLOAD, _ref(oom)])
    assert c.summary == f"Pod {POD} reports: Memory cgroup out of memory." and "unknown" not in c.summary


def test_image_pull():
    failed = _event("Failed", 'Failed to pull image "api:9.9": not found')
    c = _one([_status(ready=0, pods=[_pod(waiting="ImagePullBackOff")])], [failed])
    assert _shape(c) == ("deployment/api", "image_pull", "critical", "high", [WORKLOAD, _ref(failed)])
    assert (c.reason, c.title) == ("image_pull.not_found", "api is down: image not found")
    assert c.summary == (f"0 of 2 replicas are ready, so api is not serving. Pod {POD} cannot pull api:9.9: the "
                         "registry has no such tag or repository.")
    assert "message: not found" in c.facts and "registry: docker.io" in c.facts

    # Event only (status not fetched): a BackOff about pulling, not about restarting.
    backoff = _event("BackOff", 'Back-off pulling image "api:9.9"')
    c = _one(events=[backoff])
    assert (c.kind, c.reason, c.title) == ("image_pull", "image_pull.other", "api: image pull failing")


def test_crashloop():
    backoff = _event("BackOff", f"Back-off restarting failed container api in pod {POD}", count=9)
    pod = _pod(restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    c = _one([_status(ready=1, pods=[pod])], [backoff])
    assert _shape(c) == ("deployment/api", "crashloop", "critical", "high", [WORKLOAD, _ref(backoff)])
    assert (c.reason, c.title) == ("crashloop.exit_1", "api is degraded: application error")
    assert c.summary == (f"1 of 2 replicas are ready. Container api in pod {POD} exits with code 1 (application "
                         "error), 5 restarts.")

    # Live (rl-oomhist): after a fix rolled the crashing pod away, its BackOff is no symptom of the ready workload.
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": "api-6c5d4-new01"}}]
    assert _eval([_status()], [backoff], run=run) == []
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD}}]
    assert _one([_status()], [backoff], run=run).kind == "crashloop"


def test_startup_blip_on_a_ready_pod_is_history():
    # Live (rl-oomhist): 2 readiness failures in the first second of a pod that has been Ready since.
    readiness = _event("Unhealthy", "Readiness probe failed: connection refused")
    ready = {"type": "Ready", "status": "True", "lastTransitionTime": "2026-09-24T11:58:30Z"}
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD}, "status": {"conditions": [ready]}}]
    assert _eval([_status()], [readiness], run=run) == []
    # Failing again after it became Ready, or while the workload is short of replicas: still a symptom.
    run.pods_cache[("shop", "deployment/api")][0]["status"]["conditions"] = [
        {**ready, "lastTransitionTime": "2026-09-24T11:50:00Z"}]
    assert _one([_status()], [readiness], run=run).kind == "probe_failure"
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD}, "status": {"conditions": [ready]}}]
    assert _one([_status(ready=1)], [readiness], run=run).kind == "probe_failure"


def test_crash_loop_running_window_is_no_recovery():
    # Live (ib-probekill, V2 2026-09-25): analysed in the 2 s its restarted container ran, Ready and 1/1 ready.
    liveness = _event("Unhealthy", "Liveness probe failed: HTTP probe failed with statuscode: 404", count=7)
    backoff = _event("BackOff", f"Back-off restarting failed container api in pod {POD}", count=28)
    ready = {"type": "Ready", "status": "True", "lastTransitionTime": "2026-09-24T11:59:58Z"}
    running = {"restartCount": 8, "state": {"running": {"startedAt": "2026-09-24T11:59:58Z"}}}
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD},
                                         "status": {"conditions": [ready], "containerStatuses": [running]}}]
    status = _status(desired=1, ready=1, pods=[_pod(restarts=8, last="Completed", code=0)])
    c = _one([status], [liveness, backoff], run=run)
    assert (c.kind, c.reason) == ("crashloop", "crashloop.probe_kill")
    # The same pod up for RECOVERED_MIN_UPTIME_S or more has recovered: its loop's events are history.
    running["state"]["running"]["startedAt"] = ready["lastTransitionTime"] = "2026-09-24T11:58:30Z"
    assert _eval([status], [liveness, backoff], run=run) == []


def test_probe_failure():
    readiness = _event("Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 500", count=12)
    c = _one([_status(ready=1)], [readiness])
    assert _shape(c) == ("deployment/api", "probe_failure", "warning", "medium", [WORKLOAD, _ref(readiness)])
    assert (c.reason, c.title) == ("probe_failure.readiness", "api is degraded: failing readiness checks")
    assert c.summary == (f"1 of 2 replicas are ready. 12 readiness check failures on pod {POD}: the endpoint "
                         "returned HTTP 500. Pods that fail readiness receive no traffic.")

    # No replica ready is critical; two probe events are high confidence; no restart yet: still a probe failure.
    liveness = _event("Unhealthy", "Liveness probe failed: connection refused", obj="Pod/api-7d9f8b6c5-bbbbb", count=3)
    c = _one([_status(ready=0)], [liveness, readiness])
    assert _shape(c)[1:4] == ("probe_failure", "critical", "high")
    assert (c.reason, c.title) == ("probe_failure.liveness", "api is down: failing liveness checks")
    assert "count: 3" in c.facts


def test_probe_kill_is_crashloop():
    liveness = _event("Unhealthy", 'Liveness probe failed: Get "http://10.0.0.7:8080/nope": context deadline '
                      "exceeded (Client.Timeout exceeded while awaiting headers)", count=6)
    pod = _pod(restarts=3, last="Completed", code=0)
    c = _one([_status(ready=1, pods=[pod])], [liveness])
    assert _shape(c) == ("deployment/api", "crashloop", "critical", "high", [WORKLOAD, _ref(liveness)])
    assert (c.reason, c.title) == ("crashloop.probe_kill", "api is degraded: restarted by its liveness probe")
    assert c.summary == (f"1 of 2 replicas are ready. Container api in pod {POD} fails its liveness probe and was "
                         "restarted 3 times: no answer in time.")
    # The same event before any restart: a probe failure, not a crash loop.
    assert _one([_status(ready=1, pods=[_pod()])], [liveness]).reason == "probe_failure.liveness"
    # Readiness never restarts a container: restarts from another cause keep it a probe failure.
    readiness = _event("Unhealthy", "Readiness probe failed: connection refused", count=2)
    assert _one([_status(ready=1, pods=[pod])], [readiness]).reason == "probe_failure.readiness"


def test_scheduling():
    event = _event("FailedScheduling", "0/2 nodes are available: 2 Insufficient cpu. preemption: 0/2 nodes are "
                   "available: 2 No preemption victims found for incoming pod.")
    status = _status(desired=1, ready=0, pods=[_pod(phase="Pending")])
    status["pending"] = 1
    c = _one([status], [event])
    assert _shape(c) == ("deployment/api", "scheduling", "critical", "high", [WORKLOAD, _ref(event)])
    assert (c.reason, c.title) == ("scheduling.insufficient_cpu", "api is down: not enough CPU on any node")
    assert c.summary == ("0 of 1 replicas are ready, so api is not serving. 1 pod(s) are pending. The scheduler "
                         "reports: 0/2 nodes are available: 2 Insufficient cpu.")

    # Every replica still ready (a surge pod is pending): a warning.
    c = _one([_status(desired=2, ready=2)], [event])
    assert (c.kind, c.severity) == ("scheduling", "warning")


def test_failed_scheduling_of_a_placed_or_gone_pod_is_history():
    # Live (e2e-i-probe): the pod waited for a slot, got a node and now fails readiness. The old FailedScheduling
    # named 'nodes are full' as the cause on a workload that is not ready.
    scheduling = _event("FailedScheduling", "0/2 nodes are available: 2 Too many pods.")
    readiness = _event("Unhealthy", "Readiness probe failed: connection refused", count=16)
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD}, "spec": {"nodeName": "n1"}}]
    c = _one([_status(desired=1, ready=0, pods=[_pod()])], [scheduling, readiness], run=run)
    assert (c.kind, c.reason, c.severity) == ("probe_failure", "probe_failure.readiness", "critical")
    assert _ref(scheduling) not in [e.ref for e in c.evidence]
    # Its pod is gone: history too.
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = []
    assert _one([_status(desired=1, ready=0)], [scheduling], run=run).kind == "other"
    # Still without a node: the scheduler's report stands.
    run = _run()
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": POD}, "spec": {}}]
    c = _one([_status(desired=1, ready=0, pods=[_pod(phase="Pending")])], [scheduling, readiness], run=run)
    assert (c.kind, c.reason) == ("scheduling", "scheduling.too_many_pods")


def test_resource_pressure():
    evicted = _event("Evicted", "The node was low on resource: memory.")
    c = _one(events=[evicted])
    assert _shape(c) == ("deployment/api", "resource_pressure", "warning", "high", [WORKLOAD, _ref(evicted)])
    assert (c.reason, c.title) == ("resource_pressure.evicted_memory", "api: evicted for node memory")
    assert c.summary == f"Pod {POD} was evicted because its node ran low on memory: The node was low on resource: memory."
    # The message alone marks it too, whatever the reason.
    c = _one(events=[_event("Killing", "The node was low on resource: ephemeral-storage")])
    assert (c.kind, c.reason) == ("resource_pressure", "resource_pressure.evicted_disk")


def test_preempted_is_resource_pressure():
    pod = {**_pod(), "disruption": "PreemptionByScheduler"}
    c = _one([_status(ready=1, pods=[pod])])
    assert (c.kind, c.reason, c.severity) == ("resource_pressure", "resource_pressure.preempted", "warning")
    assert c.summary == f"1 of 2 replicas are ready. Pod {POD} was removed to make room for a higher-priority pod."
    # The scheduler's own event (a Normal event, seen when warningsOnly is off) says so too.
    preempted = _event("Preempted", "Preempted by pod 1234 on node n1")
    c = _one(events=[preempted])
    assert (c.reason, c.summary) == ("resource_pressure.preempted", f"Pod {POD} was removed to make room for a "
                                     "higher-priority pod: Preempted by pod 1234 on node n1.")


def test_failedcreate_quota_rollout():
    quota = _event("FailedCreate", 'Error creating: pods "api-7d9f8b6c5-x2k4q" is forbidden: exceeded quota: q, '
                   "requested: pods=1, used: pods=1, limited: pods=1", obj="ReplicaSet/api-7d9f8b6c5")
    # No status needed: the ReplicaSet event alone names the block.
    c = _one(events=[quota])
    assert _shape(c) == ("deployment/api", "rollout_stuck", "warning", "medium", [WORKLOAD, _ref(quota)])
    assert (c.reason, c.title) == ("rollout_stuck.quota_exceeded", "api: blocked by a resource quota")
    assert c.summary.startswith("New pods cannot be created: exceeded quota: q, requested: pods=1")
    admission = _event("FailedCreate", 'Error creating: admission webhook "validate.kyverno.svc" denied the request: '
                       "blocked", obj="ReplicaSet/api-7d9f8b6c5")
    c = _one([_status(desired=2, updated=0, ready=0)], [admission])
    assert (c.reason, c.title) == ("rollout_stuck.admission_denied", "api is down: blocked by an admission policy")
    # Any other FailedCreate is not a rollout block the rules can name: a warning card.
    other = _event("FailedCreate", 'Error creating: pods "x" is forbidden: error looking up service account',
                   obj="ReplicaSet/api-7d9f8b6c5")
    assert _one(events=[other]).reason == "other.warnings"


def test_rollout_stuck():
    deadline = {"type": "Progressing", "status": "False", "reason": "ProgressDeadlineExceeded"}
    c = _one([_status(desired=3, updated=1, ready=3, conditions=[deadline])])
    assert _shape(c) == ("deployment/api", "rollout_stuck", "warning", "medium", [WORKLOAD])
    assert (c.reason, c.summary) == ("rollout_stuck.progress_deadline",
                                     "The rollout stopped making progress: 1/3 updated, 3/3 ready.")

    # No deadline condition yet, but neither updated nor ready.
    c = _one([_status(desired=3, updated=1, ready=2)])
    assert (c.kind, c.reason, c.summary) == ("rollout_stuck", "rollout_stuck.incomplete",
                                             "The rollout is incomplete: 1/3 updated, 2/3 ready.")

    # Past the deadline, pods that cannot create their container or mount a volume still name the cause (live:
    # a missing ConfigMap / Secret volume turned into 'rollout exceeded its deadline' after 10 min).
    stuck = _pod(phase="Pending", waiting="CreateContainerConfigError")
    c = _one([_status(desired=1, updated=1, ready=0, pods=[stuck], conditions=[deadline])])
    assert (c.kind, c.reason) == ("other", "other.container_config_error")
    mount = _event("FailedMount", 'MountVolume.SetUp failed for volume "s" : secret "x" not found')
    c = _one([_status(desired=1, updated=1, ready=0, pods=[_pod(phase="Pending")], conditions=[deadline])], [mount])
    assert (c.kind, c.reason) == ("other", "other.volume_mount")


def test_paused_rollout_is_no_incident():
    # Live (e2e-i-ops): paused from creation, 0/2 updated. It waits on purpose; the recommendation says so.
    paused = {**_status(desired=2, updated=0, ready=0), "paused": True}
    assert _eval([paused]) == []
    # A symptom of its pods still is one.
    crashing = {**_status(desired=2, updated=0, ready=0, pods=[_pod(restarts=3, waiting="CrashLoopBackOff",
                                                                     last="Error", code=1)]), "paused": True}
    assert _one([crashing]).kind == "crashloop"


def test_total_outage_is_critical():
    # Live (missingcm): a missing ConfigMap at 0 ready ranked below the other outages.
    stuck = _pod(phase="Pending", waiting="CreateContainerConfigError")
    c = _one([_status(desired=1, updated=1, ready=0, pods=[stuck])])
    assert (c.kind, c.reason, c.severity) == ("other", "other.container_config_error", "critical")
    assert c.title.startswith("api is down")
    # Some replicas still serve, or none is wanted: the rule's own severity.
    assert _one([_status(desired=2, ready=1, pods=[stuck])]).severity == "warning"
    assert _eval([_status(desired=0, ready=0)]) == []


def test_config_change_regression():
    change = _change(minutes_ago=10, changes=[
        {"field": "envVarKey", "changeType": "updated", "oldValue": "db-a", "newValue": "db-b"}], change_class="config",
        incident=False)
    c = _one([_status(desired=2, ready=1)], overview=DEGRADED, changes=[change])
    assert _shape(c) == ("deployment/api", "config_change_regression", "warning", "medium", [WORKLOAD, "gen:42"])
    assert [e.type for e in c.evidence] == ["workload", "change"]
    assert c.reason == "config_change_regression.config"
    assert c.summary == ("1 of 2 replicas are ready since change gen 42. It began 10 min after change gen 42: "
                         "envVarKey db-a\u2192db-b.")
    assert "10 min after change gen 42: envVarKey db-a\u2192db-b" in c.facts
    assert (c.params["generation"], c.params["change"]) == ("42", "envVarKey db-a\u2192db-b")


def test_other_on_degraded_without_rule():
    c = _one([_status(desired=2, ready=1)])
    assert _shape(c) == ("deployment/api", "other", "warning", "low", [WORKLOAD])
    assert (c.reason, c.summary) == ("other.degraded", "1 of 2 replicas are ready and no warning was reported.")

    # Healthy replicas but an unmatched warning: other, citing the event.
    mount = _event("FailedMount", "MountVolume.SetUp failed for volume config")
    c = _one([_status()], [mount])
    assert _shape(c) == ("deployment/api", "other", "warning", "low", [WORKLOAD, _ref(mount)])
    assert c.reason == "other.volume_mount"


# ---- precedence, correlation, selection --------------------------------------------------
def test_precedence_oom_over_crashloop():
    backoff = _event("BackOff", f"Back-off restarting failed container api in pod {POD}")
    pod = _pod(restarts=6, waiting="CrashLoopBackOff", last="OOMKilled", code=137)
    c = _one([_status(ready=1, pods=[pod])], [backoff])
    # One card per subject: the stronger kind wins and keeps the other rule's evidence.
    assert _shape(c) == ("deployment/api", "oom", "critical", "high", [WORKLOAD, _ref(backoff)])


def test_change_correlation_adds_gen_ref_and_fact():
    pod = _pod(restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    c = _one([_status(ready=1, pods=[pod])], changes=[_change(minutes_ago=12)])
    assert (c.kind, [e.ref for e in c.evidence]) == ("crashloop", [WORKLOAD, "gen:42"])
    fact = "12 min after change gen 42: image api:1→api:2"
    assert fact in c.facts
    assert c.summary.endswith(f"5 restarts. It began {fact}.")

    # A non-incident rollout, config or resources change correlates too (an image tag change is 'deployment').
    for change_class in ("deployment", "config", "resources"):
        c = _one([_status(ready=1, pods=[pod])], changes=[_change(incident=False, change_class=change_class)])
        assert "gen:42" in [e.ref for e in c.evidence] and "It began 10 min after change gen 42" in c.summary

    # Older than the window, not an incident nor a correlated class, or an undatable entry: no correlation.
    for change in (_change(minutes_ago=31), _change(incident=False, change_class="scaling"),
                   {**_change(), "detectedAt": "not a time"}, {**_change(), "detectedAt": ""}):
        c = _one([_status(ready=1, pods=[pod])], changes=[change])
        assert "gen:42" not in [e.ref for e in c.evidence] and "began" not in c.summary

    # An entry without field changes still correlates; missing values read as none.
    c = _one([_status(ready=1, pods=[pod])], changes=[_change(changes=[])])
    assert "10 min after change gen 42" in c.facts
    c = _one([_status(ready=1, pods=[pod])], changes=[_change(changes=[
        {"field": "env.X", "changeType": "added", "oldValue": None, "newValue": "1"}])])
    assert "10 min after change gen 42: env.X none→1" in c.facts


def test_correlation_cites_the_cause_not_the_health_field():
    # Live (L-2b): an incident entry leads with discovery's synthetic health field; the real change follows it.
    pod = _pod(restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    health = {"field": "health", "changeType": "updated", "oldValue": "healthy", "newValue": "down"}
    command = {"field": "command[2]", "changeType": "updated", "oldValue": "sleep 36000", "newValue": "exit 1"}
    c = _one([_status(ready=1, pods=[pod])], changes=[_change(changes=[health, command])])
    assert "10 min after change gen 42: command[2] sleep 36000→exit 1" in c.facts
    assert c.params["change"] == "command[2] sleep 36000→exit 1" and "health" not in c.summary
    # Only the health field: the generation alone.
    c = _one([_status(ready=1, pods=[pod])], changes=[_change(changes=[health])])
    assert "10 min after change gen 42" in c.facts and "change" not in c.params


def test_correlation_measures_to_the_incident_start():
    # Live (R3-insights-1): crasher broke 30 s after gen 5, a manual run 5 min later said "5 min after change".
    pod = _pod(restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    first = datetime.fromtimestamp(NOW.timestamp() - 9 * 60, UTC).strftime("%Y-%m-%dT%H:%M:%SZ")
    backoff = {**_event("BackOff", f"Back-off restarting failed container api in pod {POD}"), "first": first}
    c = _one([_status(ready=1, pods=[pod])], [backoff], changes=[_change(minutes_ago=10)])
    assert "It began 1 min after change gen 42" in c.summary
    # The earliest matched event counts; one that predates the change reads as 0 min.
    later = {**_event("BackOff", "Back-off restarting failed container api", count=2), "first": LAST}
    c = _one([_status(ready=1, pods=[pod])], [later, backoff], changes=[_change(minutes_ago=10)])
    assert "It began 1 min after change gen 42" in c.summary
    c = _one([_status(ready=1, pods=[pod])], [backoff], changes=[_change(minutes_ago=1)])
    assert "It began 0 min after change gen 42" in c.summary
    # No dated symptom (a regression seen only in the replica count): the run's clock, as before.
    c = _one([_status(desired=2, ready=1)], overview=DEGRADED, changes=[_change(minutes_ago=10, incident=False,
                                                                                change_class="config")])
    assert "It began 10 min after change gen 42" in c.summary


def test_correlation_ignores_a_change_older_than_the_job():
    # Live (L-8): exporter did not hold the triggering entry yet; the newest one was an earlier fix.
    pod = _pod(restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    run = _run()
    run.min_generation = 43
    c = _one([_status(ready=1, pods=[pod])], changes=[_change()], run=run)
    assert "gen:42" not in [e.ref for e in c.evidence] and "began" not in c.summary
    run = _run()
    run.min_generation = 42
    assert "gen:42" in [e.ref for e in _one([_status(ready=1, pods=[pod])], changes=[_change()], run=run).evidence]


def test_one_restart_is_singular():
    c = _one([_status(ready=1, pods=[_pod(restarts=1, waiting="CrashLoopBackOff", last="Error", code=1)])])
    assert c.summary.endswith("exits with code 1 (application error), 1 restart.")
    liveness = _event("Unhealthy", "Liveness probe failed: connection refused", count=2)
    c = _one([_status(ready=1, pods=[_pod(restarts=1, last="Completed", code=0)])], [liveness])
    assert "was restarted 1 time:" in c.summary


def test_healthy_app_no_candidates():
    assert _eval([_status(pods=[_pod()])]) == []
    # A recent change on a healthy app is not a regression.
    assert _eval([_status()], changes=[_change()]) == []


def test_select_status_subjects_prefers_event_workloads():
    run = _run("a", "b", "c", "d")
    events = [_event("BackOff", obj="Pod/d-7d9f8b6c5-x2k4q"), _event("Unhealthy", obj="Pod/c-7d9f8b6c5-x2k4q"),
              _event("BackOff", obj="Pod/d-7d9f8b6c5-bbbbb"), _event("BackOff", obj="Pod/zzz-7d9f8b6c5-x2k4q")]
    assert rules.select_status_subjects(run, events) == [("shop", "deployment/d"), ("shop", "deployment/c"),
                                                          ("shop", "deployment/a")]
    assert rules.select_status_subjects(_run("a"), []) == [("shop", "deployment/a")]


def test_templates_within_limits():
    name = "a" * 63
    long_pod = f"{name}-7d9f8b6c5-x2k4q"
    message = "m" * 160
    run = _run(name)
    pod = _pod(name=long_pod, restarts=5, waiting="CrashLoopBackOff", last="Error", code=1)
    change = _change(changes=[{"field": "f" * 200, "changeType": "updated", "oldValue": "o" * 80, "newValue": "n" * 80}])
    (c,) = _eval([_status(name=name, ready=1, pods=[pod])], [_event("BackOff", message, obj=f"Pod/{long_pod}")],
                 changes=[change], run=run)
    assert len(c.title) <= MAX_INSIGHT_TITLE_LENGTH and len(c.summary) <= MAX_INSIGHT_SUMMARY_LENGTH
    assert all(len(v) <= 120 for v in c.params.values())


# ---- degraded inputs ------------------------------------------------------------------------
def test_k8s_unavailable_uses_overview_and_history():
    # No statuses, no events: a lone workload of a degraded app still gets a low card.
    c = _one(overview=DEGRADED)
    assert _shape(c) == ("deployment/api", "other", "warning", "low", [WORKLOAD])
    assert (c.reason, c.summary) == ("other.degraded", "Not all replicas are ready.")
    assert _one(overview=DEGRADED, changes=[_change()]).kind == "config_change_regression"
    # With several workloads nothing says which one: no card.
    assert _eval(overview=DEGRADED, run=_run("api", "web")) == []
    # An error payload reads as absent.
    assert _eval(overview={"error": "timeout", "detail": ""}) == []


def test_evidence_capped_and_backed_by_run_refs():
    events = [_event("Unhealthy", "Readiness probe failed", obj=f"Pod/api-7d9f8b6c5-{c * 5}") for c in "bcdfgh"]
    run = _run()
    (c,) = _eval([_status(ready=1)], events, changes=[_change()], run=run)
    refs = [e.ref for e in c.evidence]
    assert len(refs) == 4 and refs[:2] == [WORKLOAD, "gen:42"]

    # A ref the tools never returned is never cited.
    run = _run()
    run.refs.clear()
    assert _eval([_status(ready=1)], run=run)[0].evidence == []


def test_candidates_capped_most_severe_first():
    run = _run("a", "b", "c", "d")
    statuses = [_status(name="a", ready=1), _status(name="b", ready=1),
                _status(name="c", ready=1, pods=[_pod(name="c-7d9f8b6c5-x2k4q", waiting="CrashLoopBackOff")]),
                _status(name="d", ready=1)]
    out = _eval(statuses, run=run)
    assert [(c.subject, c.kind) for c in out] == [
        ("deployment/c", "crashloop"), ("deployment/a", "other"), ("deployment/b", "other")]
