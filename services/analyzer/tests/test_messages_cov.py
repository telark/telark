"""Checks for messages.py: every incident sub-reason through the real fast path (rules.evaluate), with a
positive, a negative and a false-positive-guard fixture each, plus the rendering limits and the params contract.

A wrong sub-reason tells an operator the wrong cause, so every reason is pinned by three
inputs shaped exactly like the tools' results. Image-pull messages are real containerd
texts longer than the 160 characters the model sees: the classification reads the full text.

Run: python -m pytest tests/test_messages_cov.py
"""

import os
import re
import sys
from datetime import UTC, datetime

import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import messages  # noqa: E402
import rules  # noqa: E402
from constants import (  # noqa: E402
    INCIDENT_DETAIL_FALLBACKS,
    INCIDENT_DETAIL_VARIANTS,
    INCIDENT_PARAM_KEYS,
    INCIDENT_TEXT,
    MAX_INSIGHT_PARAMS,
    MAX_INSIGHT_SUMMARY_LENGTH,
    MAX_INSIGHT_TITLE_LENGTH,
    REF_EVENT,
)
from models import Run  # noqa: E402

NOW = datetime(2026, 9, 24, 12, 0, tzinfo=UTC)
LAST = "2026-09-24T11:58:00Z"
POD = "api-7d9f8b6c5-x2k4q"
RS = "ReplicaSet/api-7d9f8b6c5"


def pod(name=POD, restarts=0, waiting=None, wmsg=None, last=None, code=None, container="app", init=False,
        disruption=None, phase="Running"):
    return {"name": name, "phase": phase, "restarts": restarts, "waitingReason": waiting,
            "lastTerminated": last and {"reason": last, "exitCode": code, "at": LAST}, "container": container,
            "init": init, "node": "n1", "waitingMessage": wmsg, "disruption": disruption}


def status(ready=0, desired=1, updated=None, pods=(), conditions=(), limits=None, pending=0,
           images=None):
    return {"kind": "deployment", "name": "api", "conditions": list(conditions), "desired": desired,
            "updated": desired if updated is None else updated, "ready": ready, "pods": list(pods),
            "images": ["app:1"], "limits": limits or {}, "pending": pending,
            "fullImages": images or {"app": "registry.io/team/app:1"}}


def ev(reason, message, obj=f"Pod/{POD}", count=1):
    return {"reason": reason, "object": obj, "message": message[:160], "count": count, "first": LAST, "last": LAST,
            "fullMessage": message, "namespace": "shop"}


def change(field="image", minutes_ago=10, change_class="deployment", incident=False):
    at = datetime.fromtimestamp(NOW.timestamp() - minutes_ago * 60, UTC).strftime("%Y-%m-%dT%H:%M:%SZ")
    return {"generation": 42, "detectedAt": at, "changeClass": change_class, "severity": "high",
            "isIncident": incident, "isRecovery": False,
            "changes": [{"field": field, "changeType": "updated", "oldValue": "a", "newValue": "b"}]}


def case(st=None, events=(), changes=(), degraded=False):
    return {"status": st, "events": list(events), "changes": list(changes), "degraded": degraded}


def evaluate(fixture):
    """The real fast path for one Deployment 'api' in 'shop': the candidate, or None."""
    run = Run("shop", "shop", {"resources": [{"namespace": "shop", "kind": "Deployment", "name": "api"}]}, [])
    run.refs.add("workload:deployment/api")
    if fixture["status"] is not None:
        run.status_cache[("shop", "deployment/api")] = fixture["status"]
    run.events_cache.extend(fixture["events"])
    run.refs.update(REF_EVENT.format(**e) for e in fixture["events"])
    overview = {"health": {"ready": 0, "total": 1}} if fixture["degraded"] else {"health": {}}
    out = rules.evaluate(run, overview, {"changes": fixture["changes"]}, run.status_cache, run.events_cache, NOW)
    return out[0] if out else None


def reason_of(fixture):
    c = evaluate(fixture)
    return c.reason if c else None


# ---- real kubelet / containerd / scheduler texts ------------------------------------------------------------
def pull(image, error):
    return (f'Failed to pull image "{image}": failed to pull and unpack image "{image}": failed to resolve '
            f'reference "{image}": {error}')


NOT_FOUND = pull("registry.k8s.io/pause:does-not-exist", "registry.k8s.io/pause:does-not-exist: not found")
DENIED = pull("docker.io/telarklabnope/nope:1", "pull access denied, repository does not exist or may require "
              "authorization: server message: insufficient_scope: authorization failed")
UNAUTHORIZED = pull("ghcr.io/acme/private:2.1", "failed to authorize: failed to fetch oauth token: unexpected status "
                    "from GET request to https://ghcr.io/token?scope=repository%3Aacme%2Fprivate%3Apull: "
                    "401 Unauthorized")
UNREACHABLE = pull("registry.invalid.example/x:1", 'failed to do request: Head "https://registry.invalid.example/v2/'
                   'x/manifests/1": dial tcp: lookup registry.invalid.example on 10.100.0.10:53: no such host')
RATE = pull("docker.io/library/nginx:1.27", "failed to copy: httpReadSeeker: failed open: unexpected status code "
            "https://registry-1.docker.io/v2/library/nginx/manifests/1.27: 429 Too Many Requests - Server message: "
            "toomanyrequests: You have reached your pull rate limit.")
ENOSPC = pull("quay.io/acme/app:1", "failed to extract layer sha256:4f4fb700ef54: write /var/lib/containerd/tmp: "
              "no space left on device")
X509 = pull("registry.corp.example/app:1", 'failed to do request: Head "https://registry.corp.example/v2/app/'
            'manifests/1": tls: failed to verify certificate: x509: certificate signed by unknown authority')
INVALID = ('Failed to apply default image tag "registry.k8s.io/Pause:3.10": couldn\'t parse image name '
           '"registry.k8s.io/Pause:3.10": invalid reference format: repository name (library/Pause) must be lowercase')
BACKOFF_PULL = 'Back-off pulling image "registry.k8s.io/pause:does-not-exist"'
BACKOFF_RESTART = f"Back-off restarting failed container app in pod {POD}_shop(0f3c)"
LIVE_TIMEOUT = ('Liveness probe failed: Get "http://10.0.0.7:8080/nope": context deadline exceeded '
                "(Client.Timeout exceeded while awaiting headers)")
READY_REFUSED = 'Readiness probe failed: Get "http://10.0.0.7:8081/": dial tcp 10.0.0.7:8081: connect: connection refused'
READY_404 = "Readiness probe failed: HTTP probe failed with statuscode: 404"
READY_TIMEOUT = ("Readiness probe errored: rpc error: code = DeadlineExceeded desc = failed to exec in container: "
                 "timeout 1s exceeded: context deadline exceeded")
STARTUP_REFUSED = "Startup probe failed: dial tcp 10.0.0.7:8080: connect: connection refused"
PREEMPTION = " preemption: 0/2 nodes are available: 2 No preemption victims found for incoming pod."
EVICT_MEMORY = ("The node was low on resource: memory. Threshold quantity: 100Mi, available: 60Mi. Container app was "
                "using 300Mi, request is 64Mi, has larger consumption of memory.")
EVICT_EPHEMERAL = "Pod ephemeral local storage usage exceeds the total limit of containers 10Mi."
EVICT_NODE_DISK = "The node was low on resource: ephemeral-storage. Threshold quantity: 1Gi, available: 500Mi."
EVICT_PID = "The node was low on resource: pids. Threshold quantity: 100, available: 20."
QUOTA = ('Error creating: pods "api-7d9f8b6c5-x2k4q" is forbidden: exceeded quota: q, requested: pods=1, used: '
         "pods=1, limited: pods=1")
ADMISSION = ('Error creating: admission webhook "validate.kyverno.svc-fail" denied the request: resource '
             "Deployment/shop/api was blocked due to the following policies")
SA_MISSING = 'Error creating: pods "api-7d9f8b6c5-x" is forbidden: error looking up service account shop/x'
DEADLINE = {"type": "Progressing", "status": "False", "reason": "ProgressDeadlineExceeded"}


def sched(segment, tail=PREEMPTION):
    return case(status(pods=[pod(phase="Pending")], pending=1), [ev("FailedScheduling",
                                                                     f"0/2 nodes are available: {segment}.{tail}")])


def image_case(message, waiting="ImagePullBackOff", wmsg=None, image=None):
    return case(status(pods=[pod(waiting=waiting, wmsg=wmsg)], images=image), [ev("Failed", message)])


def crash(code, last="Error", init=False, container="app", events=(), restarts=4):
    p = pod(restarts=restarts, waiting="CrashLoopBackOff", last=last, code=code, init=init, container=container)
    return case(status(pods=[p]), [ev("BackOff", BACKOFF_RESTART), *events])


def probe_case(message, restarts=0, count=5):
    return case(status(pods=[pod(restarts=restarts)]), [ev("Unhealthy", message, count=count)])


def oom_case(limits, events=(), container="app"):
    p = pod(restarts=2, waiting="CrashLoopBackOff", last="OOMKilled", code=137, container=container)
    return case(status(pods=[p], limits=limits), list(events))


def evict(message, reason="Evicted"):
    return case(status(ready=1), [ev(reason, message)])


def regression(field, **kw):
    return case(status(ready=0), changes=[change(field, **kw)])


def failed_create(message, st=None):
    return case(st, [ev("FailedCreate", message, obj=RS)])


# reason -> (positive, negative, guard): the positive must classify as the reason, the negative and the guard
# (a near miss that a sloppier matcher would take for the reason) must not.
INCIDENT_FIXTURES = {
    "image_pull.not_found": (image_case(NOT_FOUND), image_case(DENIED),
                             case(None, [ev("BackOff", BACKOFF_PULL)])),
    "image_pull.denied_or_missing": (image_case(DENIED), image_case(UNAUTHORIZED),
                                     case(None, [ev("BackOff", 'Back-off pulling image "docker.io/acme/insufficient_scope:1"')])),
    "image_pull.unauthorized": (image_case(UNAUTHORIZED), image_case(RATE),
                                image_case(pull("docker.io/acme/app:401", "docker.io/acme/app:401: not found"))),
    "image_pull.registry_unreachable": (image_case(UNREACHABLE), image_case(NOT_FOUND), image_case(X509)),
    "image_pull.invalid_name": (image_case(INVALID, waiting="InvalidImageName", wmsg=INVALID), image_case(NOT_FOUND),
                                image_case(pull("quay.io/acme/app:1", "invalid character 'x' looking for beginning of "
                                                                     "value"))),
    "image_pull.rate_limited": (image_case(RATE), image_case(UNAUTHORIZED),
                                image_case(pull("docker.io/acme/app:429", "docker.io/acme/app:429: not found"))),
    # Guard: the marker sits after character 160; only the full text names it.
    "image_pull.other": (image_case(ENOSPC), image_case(NOT_FOUND), image_case(NOT_FOUND)),
    "crashloop.probe_kill": (probe_case(LIVE_TIMEOUT, restarts=3), probe_case(LIVE_TIMEOUT, restarts=0),
                             crash(1, events=[ev("Unhealthy", READY_REFUSED)])),
    "crashloop.init_failure": (crash(1, init=True, container="init"), crash(1),
                               crash(1, container="init")),
    "crashloop.start_error": (crash(128, last="StartError"), crash(128), crash(127)),
    "crashloop.exit_0": (crash(0, last="Completed"), crash(1),
                         crash(0, last="Completed", events=[ev("Unhealthy", LIVE_TIMEOUT)])),
    "crashloop.exit_1": (crash(1), crash(2), crash(1, init=True)),
    "crashloop.exit_126": (crash(126), crash(127), crash(126, last="StartError")),
    "crashloop.exit_127": (crash(127), crash(126), crash(127, init=True)),
    "crashloop.exit_137": (crash(137), crash(139), crash(137, last="OOMKilled")),
    "crashloop.exit_139": (crash(139), crash(137), crash(139, init=True)),
    "crashloop.exit_143": (crash(143), crash(137), crash(143, events=[ev("Unhealthy", LIVE_TIMEOUT)])),
    "crashloop.exit_other": (crash(3), crash(1), crash(128, last="StartError")),
    "oom.limit": (oom_case({"app": "64Mi"}), oom_case({}), oom_case({"sidecar": "64Mi"})),
    "oom.node": (oom_case({}), oom_case({"app": "64Mi"}),
                 case(None, [ev("OOMKilled", "Memory cgroup out of memory")])),
    "probe_failure.readiness": (probe_case(READY_REFUSED), probe_case(LIVE_TIMEOUT),
                                probe_case('Liveness probe failed: Get "http://10.0.0.7:8080/readiness": context '
                                           "deadline exceeded")),
    "probe_failure.liveness": (probe_case(LIVE_TIMEOUT), probe_case(LIVE_TIMEOUT, restarts=2),
                               probe_case('Readiness probe failed: Get "http://10.0.0.7:8080/liveness": connection '
                                          "refused")),
    "probe_failure.startup": (probe_case(STARTUP_REFUSED), probe_case(STARTUP_REFUSED, restarts=1),
                              probe_case("Readiness probe failed: Get \"http://10.0.0.7:8080/startup\": connection "
                                         "refused")),
    "scheduling.insufficient_cpu": (sched("2 Insufficient cpu"), sched("2 Insufficient memory"),
                                    sched("1 Insufficient cpu, 2 node(s) had untolerated taint {dedicated: gpu}")),
    "scheduling.insufficient_memory": (sched("2 Insufficient memory"), sched("2 Insufficient cpu"),
                                       sched("2 node(s) didn't match Pod's node affinity/selector",
                                             " preemption: 0/2 nodes are available: 2 Insufficient memory.")),
    "scheduling.taints": (sched("2 node(s) had untolerated taint {node.kubernetes.io/not-ready: }"),
                          sched("2 Insufficient cpu"),
                          sched("1 Insufficient cpu, 1 node(s) had untolerated taint {dedicated: gpu}")),
    "scheduling.node_affinity": (sched("2 node(s) didn't match Pod's node affinity/selector"),
                                 sched("2 node(s) had untolerated taint {dedicated: gpu}"),
                                 sched("2 node(s) didn't match pod anti-affinity rules")),
    "scheduling.pod_anti_affinity": (sched("2 node(s) didn't match pod anti-affinity rules"),
                                     sched("2 node(s) didn't match pod topology spread constraints"),
                                     sched("2 node(s) didn't match Pod's node affinity/selector")),
    "scheduling.topology_spread": (sched("2 node(s) didn't match pod topology spread constraints"),
                                   sched("2 node(s) didn't match pod anti-affinity rules"),
                                   sched("2 node(s) didn't match Pod's node affinity/selector")),
    "scheduling.volume": (sched("pod has unbound immediate PersistentVolumeClaims"), sched("2 Too many pods"),
                          sched("1 node(s) had volume node affinity conflict, 2 Insufficient cpu")),
    "scheduling.too_many_pods": (sched("2 Too many pods"),
                                 sched("2 node(s) didn't have free ports for the requested pod ports"),
                                 sched("2 Insufficient cpu", " preemption: 0/2 nodes are available: 2 Too many pods.")),
    "scheduling.host_ports": (sched("2 node(s) didn't have free ports for the requested pod ports"),
                              sched("2 Too many pods"),
                              sched("1 node(s) didn't have free ports for the requested pod ports, 2 Insufficient "
                                    "cpu")),
    "scheduling.other": (sched("1 node(s) were unschedulable"), sched("2 Insufficient cpu"),
                         sched('persistentvolumeclaim "data" not found')),
    "resource_pressure.evicted_memory": (evict(EVICT_MEMORY), evict(EVICT_EPHEMERAL),
                                         evict("The node was low on resource: ephemeral-storage. Container app was "
                                               "using 1Gi, which exceeds its memory request")),
    "resource_pressure.evicted_ephemeral": (evict(EVICT_EPHEMERAL), evict(EVICT_MEMORY), evict(EVICT_NODE_DISK)),
    "resource_pressure.evicted_disk": (evict(EVICT_NODE_DISK), evict(EVICT_PID), evict(EVICT_EPHEMERAL)),
    "resource_pressure.evicted_pid": (evict(EVICT_PID), evict(EVICT_MEMORY),
                                      evict("The node was low on resource: memory. Container rapid-api was using "
                                            "300Mi.")),
    "resource_pressure.preempted": (case(status(ready=0, pods=[pod(disruption="PreemptionByScheduler")])),
                                    evict(EVICT_MEMORY),
                                    case(status(ready=0, pods=[pod(disruption="EvictionByEvictionAPI")]),
                                         [ev("Evicted", EVICT_MEMORY)])),
    "resource_pressure.other": (evict("The node had condition: [DiskPressure]."), evict(EVICT_MEMORY),
                                evict('Usage of EmptyDir volume "tmp" exceeds the limit "10Mi".')),
    "rollout_stuck.progress_deadline": (case(status(ready=1, desired=2, updated=1, conditions=[DEADLINE])),
                                        case(status(ready=1, desired=2, updated=1)),
                                        case(status(ready=1, desired=2, updated=1, conditions=[
                                            {"type": "Progressing", "status": "False",
                                             "reason": "ReplicaSetCreateError"}]))),
    "rollout_stuck.quota_exceeded": (failed_create(QUOTA), failed_create(ADMISSION),
                                     case(None, [ev("FailedCreate", QUOTA, obj="ReplicaSet/api-gateway-7d9f8b6c5")])),
    "rollout_stuck.admission_denied": (failed_create(ADMISSION), failed_create(QUOTA), failed_create(SA_MISSING)),
    "rollout_stuck.incomplete": (case(status(ready=1, desired=2, updated=1)),
                                 case(status(ready=1, desired=2, updated=1, conditions=[DEADLINE])),
                                 case(status(ready=2, desired=2, updated=1))),
    "config_change_regression.image": (regression("image"), regression("envVarKey"),
                                       regression("image", minutes_ago=31)),
    "config_change_regression.config": (regression("envVarKey", change_class="config"), regression("image"),
                                        regression("configMapRef", change_class="scaling")),
    "config_change_regression.resources": (regression("limitsMemory", change_class="resources"),
                                           regression("image"), regression("replicas", incident=True)),
    "config_change_regression.other": (regression("serviceAccountName", incident=True), regression("image"),
                                       regression("requestsCPU", change_class="resources")),
    "other.container_config_error": (
        case(status(pods=[pod(waiting="CreateContainerConfigError", wmsg='configmap "missing" not found')])),
        case(status(pods=[pod(waiting="CreateContainerError", wmsg="failed to reserve container name")])),
        case(status(ready=1), [ev("Failed", 'Error: configmap "missing" not found')])),
    "other.create_container_error": (
        case(status(pods=[pod(waiting="CreateContainerError", wmsg="failed to reserve container name")])),
        case(status(pods=[pod(waiting="CreateContainerConfigError", wmsg='secret "x" not found')])),
        case(status(pods=[pod(waiting="ContainerCreating")]), [ev("FailedMount", 'MountVolume.SetUp failed for volume '
                                                                  '"cfg" : secret "telark-lab-missing" not found')])),
    "other.volume_mount": (
        case(status(pods=[pod(waiting="ContainerCreating")]), [ev("FailedMount", 'MountVolume.SetUp failed for volume '
                                                                  '"cfg" : secret "telark-lab-missing" not found')]),
        case(status(ready=1), [ev("FailedCreatePodSandBox", "Failed to create pod sandbox: rpc error")]),
        case(status(ready=0), [ev("FailedMount", "MountVolume.SetUp failed", obj="Pod/api-gateway-7d9f8b6c5-x2k4q")])),
    "other.degraded": (case(status(ready=1, desired=2)), case(status(ready=1), [ev("FailedCreatePodSandBox", "x")]),
                       case(status(ready=2, desired=2))),
    "other.warnings": (case(status(ready=1), [ev("FailedCreatePodSandBox", "Failed to create pod sandbox: rpc error")]),
                       case(status(ready=1, desired=2)), case(status(ready=1), [ev("BackOff", BACKOFF_RESTART)])),
}


def test_every_incident_reason_has_fixture_triple():
    assert set(INCIDENT_TEXT) == set(INCIDENT_FIXTURES)
    assert len(INCIDENT_FIXTURES) == 52


@pytest.mark.parametrize("reason", sorted(INCIDENT_FIXTURES))
def test_positive_negative_guard(reason):
    positive, negative, guard = INCIDENT_FIXTURES[reason]
    assert reason_of(positive) == reason
    assert reason_of(negative) != reason
    assert reason_of(guard) != reason


def test_guards_land_where_they_belong():
    """A few guards pin their exact outcome: the precedence they protect is the point."""
    fixtures = {r: f[2] for r, f in INCIDENT_FIXTURES.items()}
    assert reason_of(fixtures["image_pull.other"]) == "image_pull.not_found"
    assert reason_of(fixtures["image_pull.not_found"]) == "image_pull.other"
    assert reason_of(fixtures["image_pull.unauthorized"]) == "image_pull.not_found"
    assert reason_of(fixtures["crashloop.probe_kill"]) == "crashloop.exit_1"
    assert reason_of(fixtures["crashloop.exit_137"]) == "oom.node"
    assert reason_of(fixtures["crashloop.exit_143"]) == "crashloop.probe_kill"
    assert reason_of(fixtures["scheduling.insufficient_cpu"]) == "scheduling.taints"
    assert reason_of(fixtures["scheduling.taints"]) == "scheduling.insufficient_cpu"
    assert reason_of(fixtures["resource_pressure.evicted_ephemeral"]) == "resource_pressure.evicted_disk"
    assert reason_of(fixtures["rollout_stuck.quota_exceeded"]) is None
    assert reason_of(fixtures["rollout_stuck.admission_denied"]) == "other.warnings"
    assert reason_of(fixtures["other.warnings"]) == "crashloop.exit_other"


PROBE_FAILURES = [
    (READY_REFUSED, {"failure": "refused", "port": "8081"}, "connection refused on port 8081"),
    (READY_404, {"failure": "http_status", "status": "404"}, "the endpoint returned HTTP 404"),
    (READY_TIMEOUT, {"failure": "timeout", "timeout": "1"}, "no answer within 1s"),
    (LIVE_TIMEOUT.replace("Liveness", "Readiness"), {"failure": "timeout"}, "no answer in time"),
    ("Readiness probe failed: command 'cat /secret-token-abc' exited with 1", {"failure": "command"},
     "the check command failed"),
    ("Readiness probe failed: dial tcp: connection refused", {"failure": "refused"}, "connection refused"),
    ("Readiness probe failed: service not ready yet", {"failure": "other"}, "service not ready yet"),
    ("Readiness probe failed:", {"failure": "other"}, "the check failed"),
]


@pytest.mark.parametrize("message,expected,text", PROBE_FAILURES)
def test_probe_failure_texts(message, expected, text):
    c = evaluate(probe_case(message))
    assert {k: c.params[k] for k in expected} == expected
    assert f": {text}. Pods that fail readiness" in c.summary
    if expected["failure"] == "command":
        # The exec output is the application's own text (it may hold anything): never stored or stated.
        assert "message" not in c.params and "secret-token-abc" not in c.summary + " ".join(c.facts)


def _render_values(c):
    return {**messages._derived(c.params), "workload": "api", "whatSentence": "x"}


def test_every_template_placeholder_is_a_guaranteed_param():
    for reason, (positive, _negative, _guard) in INCIDENT_FIXTURES.items():
        c = evaluate(positive)
        # The reason's own detail (or its declared variant), never the generic fallback.
        own = (INCIDENT_TEXT[reason][1], *INCIDENT_DETAIL_VARIANTS.get(reason, ()))
        assert messages.pick(own, _render_values(c)), reason
        assert c.summary and not re.search(r"\{[A-Za-z]+\}|unknown|None", c.summary), reason
        assert set(c.params) <= set(INCIDENT_PARAM_KEYS), reason
        # Without a status read (beyond FAST_STATUS_MAX): the events alone still render a clean text.
        for kind in {c.kind} if positive["events"] else ():
            reason_e, params = messages.classify(kind, None, [], positive["events"], None, None)
            title, summary = messages.render_incident(reason_e, {**params, "workload": "api"}, "api", None, "")
            assert summary and title.startswith("api: "), reason
            assert not re.search(r"\{[A-Za-z]+\}|unknown|None", title + summary), (reason, summary)


def test_fallbacks_never_leave_placeholders():
    assert messages.fill(INCIDENT_DETAIL_FALLBACKS, {"whatSentence": "Out of memory"}) == "Out of memory."
    assert messages.what("unknown.reason", {}) == ""
    # A 'what' whose placeholder is missing keeps its words, never the brace.
    assert messages.what("crashloop.probe_kill", {}) == "restarted by its"


def test_render_limits():
    workload = "w" * 30
    for reason, (positive, _n, _g) in INCIDENT_FIXTURES.items():
        c = evaluate(positive)
        title, summary = messages.render_incident(reason, c.params, workload, positive["status"], "")
        assert len(title) <= 80, (reason, title)
        assert len(summary) <= 240, (reason, len(summary), summary)
        # Every param at its cap: the hard caps still hold.
        huge = {k: v if k == "failure" else "x" * 120 for k, v in c.params.items()}
        title, summary = messages.render_incident(reason, huge, "w" * 63, positive["status"], " It began " + "y" * 200)
        assert len(title) <= MAX_INSIGHT_TITLE_LENGTH and len(summary) <= MAX_INSIGHT_SUMMARY_LENGTH


def test_state_and_impact():
    down, partial, full = status(ready=0, desired=2), status(ready=1, desired=2), status(ready=2, desired=2)
    params = {"pod": "p", "message": "m"}
    assert messages.render_incident("other.volume_mount", params, "web", down, "") == (
        "web is down: a volume cannot be mounted",
        "0 of 2 replicas are ready, so web is not serving. Pod p is waiting for a volume: m.")
    assert messages.render_incident("other.volume_mount", params, "web", partial, "")[0] == (
        "web is degraded: a volume cannot be mounted")
    title, summary = messages.render_incident("other.volume_mount", params, "web", full, "")
    assert title == "web: a volume cannot be mounted" and summary.startswith("2 of 2 replicas are ready.")
    # Scaled to zero, or no status read: neither state nor impact.
    assert messages.render_incident("other.volume_mount", params, "web", status(ready=0, desired=0), "") == (
        "web: a volume cannot be mounted", "Pod p is waiting for a volume: m.")


def test_params_bounded():
    largest = 0
    for reason, (positive, _n, _g) in INCIDENT_FIXTURES.items():
        fixture = dict(positive, changes=[change("image")], status=positive["status"])
        c = evaluate(fixture)
        assert len(c.params) <= MAX_INSIGHT_PARAMS and all(len(v) <= 120 for v in c.params.values()), reason
        largest = max(largest, len(c.params))
    assert largest <= MAX_INSIGHT_PARAMS
    # probe_kill with a correlated change carries the largest set; nothing is dropped.
    c = evaluate(dict(INCIDENT_FIXTURES["crashloop.probe_kill"][0], changes=[change("image")]))
    assert {"workload", "namespace", "pod", "container", "restarts", "probe", "count", "failure", "message", "ready",
            "desired", "generation", "change"} <= set(c.params)
    long = messages.bounded({"message": "m" * 500, "bogus": "x", "pod": None, "container": ""})
    assert long == {"message": "m" * 119 + "…"}


def test_registry_of_invalid_reference():
    assert messages.registry_of("registry.k8s.io/Pause:3.10") == "registry.k8s.io"
    assert messages.registry_of("Acme/App") == "docker.io"
    assert messages.registry_of("nginx") == "docker.io"


def test_image_from_spec_when_no_message_names_it():
    st = status(pods=[pod(waiting="ErrImagePull", wmsg="rpc error: code = NotFound desc = not found")],
                images={"app": "ghcr.io/acme/app:9"})
    c = evaluate(case(st))
    assert (c.reason, c.params["image"], c.params["registry"]) == ("image_pull.not_found", "ghcr.io/acme/app:9",
                                                                   "ghcr.io")


def test_scheduler_reason_ties_and_counts():
    assert messages.scheduling_reason("0/5 nodes are available: 1 Insufficient cpu, 3 Insufficient memory.") == (
        "scheduling.insufficient_memory")
    assert messages.scheduling_reason("no nodes available to schedule pods") == "scheduling.other"
    assert messages.change_reason("chart.version") == "config_change_regression.image"


def test_live_api_messages_are_cleaned():
    # Messages captured on telark-dev (k8s 1.34): the aggregation prefix and the never-created pod name go, and the
    # scheduler's DRA note does not end up in the summary.
    combined = "(combined from similar events): " + QUOTA
    c = evaluate(failed_create(combined, status(ready=1, desired=2, updated=1)))
    assert c.reason == "rollout_stuck.quota_exceeded"
    assert c.params["message"] == "exceeded quota: q, requested: pods=1, used: pods=1, limited: pods=1"
    assert evaluate(failed_create(ADMISSION)).params["message"].startswith('admission webhook "validate.kyverno')
    anti = sched("2 node(s) didn't match pod anti-affinity rules",
                 tail=" no new claims to deallocate," + PREEMPTION)
    c = evaluate(anti)
    assert c.reason == "scheduling.pod_anti_affinity"
    assert c.params["message"] == "0/2 nodes are available: 2 node(s) didn't match pod anti-affinity rules"
