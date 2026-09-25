"""Checks for recommendations.py: all 60 v1 rules, each with a positive, a negative and a false-positive-guard
fixture, plus the registry, family completeness, the hardened base, severities, suggestions and secret hygiene.

A wrong recommendation costs trust in every other one, so each rule is pinned by three
inputs built from the V3 hardened base (tests/specs.py) with one field changed.

Run: python -m pytest tests/test_recommendations_cov.py
"""

import copy
import os
import sys
from datetime import timedelta

import pytest

sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import insights  # noqa: E402
import messages  # noqa: E402
import recommendations as R  # noqa: E402
from constants import RECOMMENDATION_TEXT, RFC3339_FORMAT  # noqa: E402
from models import AppInsights, Insight  # noqa: E402
from specs import (  # noqa: E402
    ALL,
    IMAGE,
    NOW,
    NS,
    build,
    container,
    hardened,
    hpa,
    netpol,
    pdb,
    pods,
    service,
    usage,
    workload,
)

SUBJECT = "deployment/web"


def ts(**delta):
    return (NOW - timedelta(**delta)).strftime(RFC3339_FORMAT)


def c_with(**over):
    return build([workload(containers=[container(**over)])])


def sc_with(**over):
    return c_with(securityContext={**container()["securityContext"], **over})


def sc_without(*keys, **over):
    ctx = {k: v for k, v in container()["securityContext"].items() if k not in keys}
    return c_with(securityContext={**ctx, **over})


def edit_spec(**over):
    obj = workload()
    obj["spec"].update(over)
    return obj


def res(**resources):
    return c_with(resources=resources)


def used(cpu_m, mem_b, n=13, resources=None, **kw):
    obj = workload(containers=[container(resources=resources)] if resources else None)
    samples = usage([{"app": (cpu_m, mem_b)}] * n)
    return build([obj], usage_by={(NS, SUBJECT): samples}, **kw)


MI = 2**20
BIG = {"requests": {"cpu": "150m", "memory": "160Mi"}, "limits": {"cpu": "300m", "memory": "320Mi"}}


def one_node(**pod_over):
    obj = workload(topologySpreadConstraints=[], **pod_over)
    return build([obj], pod_lists=[pods(obj, nodes=("n1",))])


def strategy(kind, **pod_over):
    obj = workload(**pod_over)
    obj["spec"]["strategy"] = {"type": kind}
    return build([obj])


def liveness_events(restarts=2, startup=False):
    obj = workload(containers=[container(startupProbe={"tcpSocket": {"port": "http"}} if startup else None)])
    events = [{"reason": "Unhealthy", "object": "Pod/web-7d9f8b6c5-x2k4b", "message": "Liveness probe failed: dial tcp",
               "count": 4, "first": ts(minutes=5), "last": ts(minutes=1), "namespace": NS}]
    return build([obj], events=events, pod_lists=[pods(obj, restarts=restarts)])


def oom_doc(limit="64Mi", status="resolved", resolved=None, reason="oom.limit", kind="oom"):
    return AppInsights(insights=[Insight(
        id="i", kind=kind, subject=SUBJECT, status=status, category="incident", reason=reason,
        params={"container": "app", "limit": limit}, firstSeenAt=ts(days=3), lastSeenAt=ts(days=2),
        resolvedAt=resolved if resolved is not None else (ts(days=2) if status == "resolved" else ""))])


def with_hpa(*args, obj=None, **kw):
    return build([obj or workload()], hpas=[hpa(*args, **kw)])


LIMITED = {"currentReplicas": 2, "conditions": [
    {"type": "ScalingLimited", "status": "True", "reason": "TooManyReplicas"}]}
INACTIVE = {"currentReplicas": 1, "conditions": [
    {"type": "ScalingActive", "status": "False", "reason": "FailedGetResourceMetric"}]}


def env(entries):
    return c_with(env=entries)


def selector_case(pods_found, selector=None):
    return build(services=[service("web"), service("web-a", selector=selector or {"app.kubernetes.io/name": "nothing"})],
                 extra_resources=[{"namespace": NS, "kind": "Service", "name": "web-a"}],
                 selector_pods={(NS, "web"): 1, (NS, "web-a"): pods_found})


def churn(count, first_days=10):
    app = hardened().app
    app["metrics"] = {"derived": {"firstChangeDetectedAt": ts(days=first_days)}}
    app["history"] = {"changeLog": [{"generation": i, "changeClass": "deployment", "detectedAt": ts(hours=i)}
                                    for i in range(count)]}
    return build(app=app)


def rolled(*days):
    app = hardened().app
    app["rollbacks"] = [{"id": f"r{d}", "targetSnapshotId": f"s{d}", "triggeredAt": ts(days=d)} for d in days]
    return build(app=app)


def prod(plans=(), ns="shop-prod", image=IMAGE, envs=None):
    obj = workload(containers=[container(image=image)])
    obj["metadata"]["namespace"] = ns
    return build([obj], ns=ns, plans=list(plans), envs=envs)


def plan(phase="active", mode="enforce", scope=None, **kw):
    return {"id": f"p-{phase}-{mode}", "name": f"plan-{mode}", "phase": phase, "mode": mode,
            "scope": scope or {"type": "applications", "applicationIds": ["web"]}, **kw}


def skew(second_image):
    a, b = workload(), workload(containers=[container(image=second_image)])
    b["metadata"]["namespace"] = "shop-b"
    return build([a, b], namespaces=[NS, "shop-b"])


def config_mount(sub_path=True, volume=None):
    mount = {"name": "cfg", "mountPath": "/etc/app/app.yaml", **({"subPath": "app.yaml"} if sub_path else {})}
    obj = workload(containers=[container(volumeMounts=[{"name": "tmp", "mountPath": "/tmp"}, mount])],
                   volumes=[{"name": "tmp", "emptyDir": {}}, volume or {"name": "cfg", "configMap": {"name": "c"}}])
    return build([obj])


# reason -> (positive, negative, guard) builders: the positive must find the reason, the negative and the guard
# (a near miss a sloppier rule would flag) must not.
CASES = {
    "reliability.single_replica": (lambda: build([workload(replicas=1)]), hardened,
                                   lambda: build([workload(replicas=0)], pod_lists=[[]])),
    "reliability.no_pdb": (lambda: build(pdbs=[]), hardened, lambda: build([workload(replicas=1)], pdbs=[])),
    "reliability.pdb_blocks_eviction": (
        lambda: build(pdbs=[pdb(minAvailable=2, status={"disruptionsAllowed": 0, "currentHealthy": 2, "expectedPods": 2})]),
        hardened,
        lambda: build(pdbs=[pdb(minAvailable=2, status={"disruptionsAllowed": 0, "currentHealthy": 1, "expectedPods": 2})])),
    "reliability.no_readiness_probe": (lambda: c_with(readinessProbe=None), hardened,
                                       lambda: build([workload(containers=[container(readinessProbe=None)])],
                                                     services=[])),
    "reliability.no_liveness_probe": (lambda: c_with(livenessProbe=None), hardened,
                                      lambda: build([workload(initContainers=[container("init", livenessProbe=None)])])),
    "reliability.liveness_same_as_readiness": (
        lambda: c_with(livenessProbe={"httpGet": {"path": "/", "port": "http"}, "periodSeconds": 5}), hardened,
        lambda: c_with(livenessProbe={"httpGet": {"path": "/", "port": "http"}, "periodSeconds": 5,
                                      "failureThreshold": 6})),
    "reliability.no_startup_probe": (liveness_events, lambda: liveness_events(startup=True),
                                     lambda: liveness_events(restarts=0)),
    "reliability.replicas_same_node": (one_node, hardened,
                                       lambda: one_node(nodeSelector={"kubernetes.io/hostname": "n1"})),
    "reliability.rollout_all_at_once": (
        lambda: strategy("Recreate"), hardened,
        lambda: strategy("Recreate", volumes=[{"name": "data", "persistentVolumeClaim": {"claimName": "d"}}])),
    "reliability.short_grace_period": (lambda: build([workload(terminationGracePeriodSeconds=0)]),
                                       lambda: build([workload(terminationGracePeriodSeconds=2)]),
                                       lambda: build([workload(terminationGracePeriodSeconds=None)])),
    "resources.no_requests": (lambda: res(), hardened, lambda: res(limits={"cpu": "50m", "memory": "32Mi"})),
    "resources.no_memory_limit": (
        lambda: res(requests={"cpu": "20m", "memory": "32Mi"}, limits={"cpu": "100m"}), hardened,
        lambda: _limitrange_defaulted()),
    "resources.limits_without_requests": (
        lambda: res(limits={"cpu": "50m", "memory": "32Mi"}), hardened,
        lambda: res(requests={"cpu": "50m", "memory": "32Mi"}, limits={"cpu": "50m", "memory": "32Mi"})),
    "resources.memory_near_limit": (lambda: used(10, 60 * MI), lambda: used(10, 30 * MI),
                                    lambda: used(10, 63 * MI, n=5)),
    "resources.cpu_near_limit": (lambda: used(95, 20 * MI), lambda: used(20, 20 * MI),
                                 lambda: used(95, 20 * MI, resources={"requests": {"cpu": "100m", "memory": "32Mi"},
                                                                      "limits": {"memory": "64Mi"}})),
    "resources.overprovisioned": (lambda: used(10, 20 * MI, resources=BIG), lambda: used(140, 150 * MI, resources=BIG),
                                  lambda: used(1, 1 * MI)),
    "resources.underprovisioned": (lambda: used(10, 60 * MI), lambda: used(10, 30 * MI),
                                   lambda: used(10, 60 * MI, resources={"limits": {"cpu": "100m", "memory": "64Mi"}})),
    "resources.oom_history": (lambda: build(doc=oom_doc()), lambda: build(doc=oom_doc(limit="32Mi")),
                              lambda: build(doc=oom_doc(status="open"))),
    "scaling.hpa_min_equals_max": (lambda: with_hpa(min_replicas=2, max_replicas=2), lambda: with_hpa(),
                                   lambda: build(hpas=[hpa("other", 2, 2)])),
    "scaling.hpa_missing_requests": (
        lambda: with_hpa(obj=workload(containers=[container(resources={"requests": {"memory": "32Mi"}})])),
        lambda: with_hpa(),
        lambda: with_hpa(obj=workload(containers=[container(resources={"requests": {"memory": "32Mi"}})]),
                         metrics=[{"type": "Resource", "resource": {"name": "cpu", "target": {
                             "type": "AverageValue", "averageValue": "50m"}}}])),
    "scaling.hpa_at_max": (lambda: with_hpa(status=LIMITED), lambda: with_hpa(status={"currentReplicas": 2}),
                           lambda: with_hpa(min_replicas=2, max_replicas=2, status=LIMITED)),
    "scaling.no_hpa_sustained_load": (lambda: used(18, 20 * MI), lambda: used(5, 20 * MI),
                                      lambda: used(18, 20 * MI, hpas=[hpa()])),
    "security.privileged": (lambda: sc_without("allowPrivilegeEscalation", privileged=True), hardened,
                            lambda: sc_with(privileged=False)),
    "security.privilege_escalation_allowed": (lambda: sc_without("allowPrivilegeEscalation"), hardened,
                                              lambda: sc_without("allowPrivilegeEscalation", privileged=True)),
    "security.runs_as_root": (lambda: sc_with(runAsUser=0), hardened,
                              lambda: build([workload(containers=[container(securityContext={
                                  k: v for k, v in container()["securityContext"].items()
                                  if k not in ("runAsUser", "runAsNonRoot")})],
                                  securityContext={"runAsNonRoot": True})])),
    "security.writable_root_fs": (lambda: sc_with(readOnlyRootFilesystem=False), hardened, lambda: _sidecar_pods()),
    "security.added_capabilities": (lambda: sc_with(capabilities={"drop": ["ALL"], "add": ["NET_ADMIN"]}), hardened,
                                    lambda: sc_without("allowPrivilegeEscalation", privileged=True,
                                                       capabilities={"add": ["SYS_ADMIN"]})),
    "security.host_namespaces": (lambda: build([workload(hostNetwork=True)]), hardened,
                                 lambda: build([workload(hostNetwork=False, hostPID=False)])),
    "security.host_path": (
        lambda: build([workload(volumes=[{"name": "host", "hostPath": {"path": "/var"}}],
                                containers=[container(volumeMounts=[{"name": "host", "mountPath": "/h"}])])]),
        hardened, lambda: build([workload(volumes=[{"name": "host", "emptyDir": {}}])])),
    "security.default_service_account": (lambda: build([workload(serviceAccountName="default")]), hardened,
                                         lambda: build([workload(serviceAccountName="defaults")])),
    "security.token_automount": (
        lambda: _token_pods(), hardened,
        lambda: _token_pods(audience="sts.amazonaws.com")),
    "security.secrets_in_env": (
        lambda: env([{"name": "DB_URL", "valueFrom": {"secretKeyRef": {"name": "db", "key": "url"}}}]), hardened,
        lambda: env([{"name": "DB_URL", "valueFrom": {"configMapKeyRef": {"name": "db", "key": "url"}}}])),
    "security.plaintext_secret_env": (lambda: env([{"name": "DB_PASSWORD", "value": "hunter2-lab"}]),
                                      lambda: env([{"name": "DB_HOST", "value": "db.shop"}]),
                                      lambda: env([{"name": "JWT_SECRET_NAME", "value": "my-secret"}])),
    "images.mutable_tag": (lambda: c_with(image="nginxinc/nginx-unprivileged:latest"), hardened,
                           lambda: c_with(image="nginxinc/nginx-unprivileged@" + IMAGE.split("@")[1])),
    "images.pull_policy_mismatch": (
        lambda: c_with(image="nginxinc/nginx-unprivileged:latest", imagePullPolicy="IfNotPresent"),
        lambda: c_with(image="nginxinc/nginx-unprivileged:latest", imagePullPolicy="Always"),
        lambda: c_with(image="nginxinc/nginx-unprivileged:1.27", imagePullPolicy="IfNotPresent")),
    "config.duplicate_env": (lambda: env([{"name": "FOO", "value": "1"}, {"name": "FOO", "value": "2"}]),
                             lambda: env([{"name": "FOO", "value": "1"}]),
                             lambda: c_with(env=[{"name": "FOO", "value": "1"}],
                                            envFrom=[{"configMapRef": {"name": "has-foo"}}])),
    "networking.service_selector_mismatch": (lambda: selector_case(0), lambda: selector_case(1),
                                             lambda: selector_case(0, selector={"app.kubernetes.io/name": "web"})),
    "networking.service_port_mismatch": (
        lambda: build(services=[service(target="httpx")]), hardened,
        lambda: build([workload(containers=[container(ports=[], readinessProbe=None, livenessProbe=None)])],
                      services=[service(target=9090)])),
    "networking.no_network_policy": (lambda: build(netpols=[]), hardened,
                                     lambda: build(netpols=[netpol("all", selector={})])),
    "change_risk.high_velocity": (lambda: churn(150), lambda: churn(10), lambda: churn(150, first_days=1)),
    "change_risk.frequent_rollbacks": (lambda: rolled(1, 2), lambda: rolled(1), lambda: rolled(10, 12)),
    "protection.production_uncovered": (lambda: prod(), lambda: prod([plan()]), lambda: prod(ns="products")),
    "protection.production_audit_only": (lambda: prod([plan(mode="audit")]), lambda: prod([plan()]),
                                         lambda: prod([plan(phase="scheduled", mode="audit")])),
    "consistency.image_skew": (lambda: skew(IMAGE.replace("a" * 64, "b" * 64)), lambda: skew(IMAGE),
                               lambda: _skew_replicas_only()),
    "reliability.revision_history_zero": (lambda: build([edit_spec(revisionHistoryLimit=0)]), hardened,
                                          lambda: _statefulset(revisionHistoryLimit=0)),
    "reliability.deployment_paused": (lambda: build([edit_spec(paused=True)]), hardened,
                                      lambda: build([edit_spec(paused=False)])),
    "reliability.liveness_single_failure": (
        lambda: c_with(livenessProbe={"tcpSocket": {"port": "http"}, "failureThreshold": 1}), hardened,
        lambda: c_with(startupProbe={"tcpSocket": {"port": "http"}, "failureThreshold": 1})),
    "reliability.probe_port_undeclared": (
        lambda: c_with(readinessProbe={"httpGet": {"path": "/", "port": "healthz"}}), hardened,
        lambda: c_with(readinessProbe={"httpGet": {"path": "/", "port": 9999}})),
    "reliability.pdb_blocks_at_min_scale": (
        lambda: build(hpas=[hpa(min_replicas=2, max_replicas=4)], pdbs=[pdb(minAvailable=2, status={
            "disruptionsAllowed": 1, "currentHealthy": 3, "expectedPods": 3})]),
        lambda: build(hpas=[hpa(min_replicas=2, max_replicas=4)]),
        lambda: build([workload(replicas=3)], pdbs=[pdb(minAvailable=2, status={
            "disruptionsAllowed": 1, "currentHealthy": 3, "expectedPods": 3})])),
    "scaling.hpa_inactive": (
        lambda: with_hpa(status=INACTIVE),
        lambda: with_hpa(status={"currentReplicas": 1, "conditions": [
            {"type": "ScalingActive", "status": "True", "reason": "ValidMetricFound"}]}),
        lambda: with_hpa(obj=workload(containers=[container(resources={"requests": {"memory": "32Mi"}})]),
                         status=INACTIVE)),
    "security.seccomp_unset": (lambda: sc_without("seccompProfile"), hardened,
                               lambda: build([workload(containers=[container(securityContext={
                                   k: v for k, v in container()["securityContext"].items() if k != "seccompProfile"})],
                                   securityContext={"seccompProfile": {"type": "RuntimeDefault"}})])),
    "security.capabilities_not_dropped": (lambda: sc_with(capabilities={"drop": []}), hardened,
                                          lambda: sc_without("allowPrivilegeEscalation", privileged=True,
                                                             capabilities={})),
    "security.host_port": (
        lambda: c_with(ports=[{"name": "http", "containerPort": 8080, "hostPort": 18080}]), hardened,
        lambda: build([workload(hostNetwork=True, containers=[container(ports=[
            {"name": "http", "containerPort": 8080, "hostPort": 8080}])])])),
    "security.run_as_root_group": (lambda: sc_with(runAsGroup=0), lambda: sc_with(runAsGroup=1000), hardened),
    "security.proc_mount_unmasked": (lambda: sc_with(procMount="Unmasked"), lambda: sc_with(procMount="Default"),
                                     hardened),
    "images.pull_policy_never": (
        lambda: c_with(imagePullPolicy="Never"), hardened,
        lambda: c_with(image="nginxinc/nginx-unprivileged:latest", imagePullPolicy="Never")),
    "images.digest_not_pinned_production": (lambda: prod([plan()], image="nginxinc/nginx-unprivileged:1.27"),
                                            lambda: prod([plan()]),
                                            lambda: prod([plan()], ns="shop", image="nginxinc/nginx-unprivileged:1.27")),
    "config.subpath_no_reload": (config_mount, lambda: config_mount(sub_path=False),
                                 lambda: config_mount(volume={"name": "cfg", "emptyDir": {}})),
    "scaling.hpa_scale_down_disabled": (
        lambda: with_hpa(min_replicas=2, max_replicas=3, behavior={"scaleDown": {"selectPolicy": "Disabled"}}),
        lambda: with_hpa(min_replicas=2, max_replicas=3, behavior={"scaleDown": {"selectPolicy": "Max"}}),
        lambda: with_hpa(min_replicas=2, max_replicas=3, behavior={"scaleUp": {"selectPolicy": "Disabled"}})),
    "networking.network_policy_allows_all": (lambda: build(netpols=[netpol(ingress=[{}])]), hardened,
                                             lambda: build(netpols=[netpol(ingress=[{"ports": [{"port": 8080}]}])])),
}


def _limitrange_defaulted():
    """The template sets no memory limit; a LimitRange defaulted one into the running pods."""
    obj = workload(containers=[container(resources={"requests": {"cpu": "20m", "memory": "32Mi"}})])
    running = pods(obj)
    for p in running:
        p["spec"]["containers"][0]["resources"]["limits"] = {"memory": "256Mi"}
    return build([obj], pod_lists=[running])


def _sidecar_pods():
    """An injected sidecar (present in the pods only) is never judged."""
    obj = workload()
    running = pods(obj)
    for p in running:
        p["spec"]["containers"].append({"name": "istio-proxy", "image": "istio/proxyv2:1.22"})
    return build([obj], pod_lists=[running])


def _token_pods(audience=None):
    obj = workload()
    running = pods(obj, token=True)
    if audience:
        for p in running:
            p["spec"]["volumes"][-1]["projected"]["sources"][0]["serviceAccountToken"]["audience"] = audience
    return build([obj], pod_lists=[running])


def _skew_replicas_only():
    a, b = workload(), workload(replicas=5, containers=[container(resources=BIG)])
    b["metadata"]["namespace"] = "shop-b"
    return build([a, b], namespaces=[NS, "shop-b"])


def _statefulset(**spec):
    obj = workload(kind="StatefulSet")
    obj["spec"].update(spec)
    return build([obj])


V1_RULES = sorted([
    "reliability.single_replica", "reliability.no_pdb", "reliability.pdb_blocks_eviction",
    "reliability.no_readiness_probe", "reliability.no_liveness_probe", "reliability.liveness_same_as_readiness",
    "reliability.no_startup_probe", "reliability.replicas_same_node", "reliability.rollout_all_at_once",
    "reliability.short_grace_period", "resources.no_requests", "resources.no_memory_limit",
    "resources.limits_without_requests", "resources.memory_near_limit", "resources.cpu_near_limit",
    "resources.overprovisioned", "resources.underprovisioned", "resources.oom_history", "scaling.hpa_min_equals_max",
    "scaling.hpa_missing_requests", "scaling.hpa_at_max", "scaling.no_hpa_sustained_load", "security.privileged",
    "security.privilege_escalation_allowed", "security.runs_as_root", "security.writable_root_fs",
    "security.added_capabilities", "security.host_namespaces", "security.host_path",
    "security.default_service_account", "security.token_automount", "security.secrets_in_env",
    "security.plaintext_secret_env", "images.mutable_tag", "images.pull_policy_mismatch", "config.duplicate_env",
    "networking.service_selector_mismatch", "networking.service_port_mismatch", "networking.no_network_policy",
    "change_risk.high_velocity", "change_risk.frequent_rollbacks", "protection.production_uncovered",
    "protection.production_audit_only", "consistency.image_skew",
    # S9b
    "reliability.revision_history_zero", "reliability.deployment_paused", "reliability.liveness_single_failure",
    "reliability.probe_port_undeclared", "reliability.pdb_blocks_at_min_scale", "scaling.hpa_inactive",
    "security.seccomp_unset", "security.capabilities_not_dropped", "security.host_port",
    "security.run_as_root_group", "security.proc_mount_unmasked", "images.pull_policy_never",
    "images.digest_not_pinned_production", "config.subpath_no_reload", "scaling.hpa_scale_down_disabled",
    "networking.network_policy_allows_all",
])


def _reasons(inputs):
    return {f.reason for f in R.evaluate(inputs, NOW)[0]}


def test_registry_has_exactly_the_60_v1_rules():
    assert len(V1_RULES) == 60
    assert V1_RULES == sorted(R.RULES) == sorted(RECOMMENDATION_TEXT) == sorted(CASES)
    for reason, rule in R.RULES.items():
        assert rule.families <= {"W", "S", "P", "H", "N", "U", "A", "D", "X"} and rule.families, reason
        assert reason.partition(".")[0] in {"reliability", "resources", "scaling", "security", "images", "config",
                                            "networking", "change_risk", "protection", "consistency"}


@pytest.mark.parametrize("reason", sorted(CASES))
def test_positive_exact(reason):
    findings, evaluated = R.evaluate(CASES[reason][0](), NOW)
    found = [f for f in findings if f.reason == reason]
    assert found, reason
    f = found[0]
    assert (reason, f.namespace, f.key) in evaluated and f.kind == reason.partition(".")[0]
    # The rendered text is complete: every placeholder is a param (or derived from one).
    title, summary = messages.render_recommendation(reason, f.params)
    assert title and summary and "{" not in title + summary, (reason, summary)
    assert all(len(v) <= 120 for v in f.params.values()) and len(f.params) <= 16
    assert 1 <= len(f.evidence) <= 4 and all(e.type and e.ref for e in f.evidence)


@pytest.mark.parametrize("reason", sorted(CASES))
def test_negative_absent(reason):
    findings, evaluated = R.evaluate(CASES[reason][1](), NOW)
    assert reason not in {f.reason for f in findings}
    # Evaluated and not found: this is what resolves an existing card.
    assert reason in {key[0] for key in evaluated}, reason


@pytest.mark.parametrize("reason", sorted(CASES))
def test_guard_absent(reason):
    assert reason not in _reasons(CASES[reason][2]())


def test_hardened_app_has_no_findings():
    findings, evaluated = R.evaluate(hardened(), NOW)
    assert findings == []
    # The hardened base in production, covered by an enforce plan, is still clean.
    assert R.evaluate(prod([plan()]), NOW)[0] == []


@pytest.mark.parametrize("reason", sorted(CASES))
def test_incomplete_family_neither_finds_nor_evaluates(reason):
    rule = R.RULES[reason]
    for family in rule.families:
        inputs = CASES[reason][0]()
        inputs.complete.discard(family)
        findings, evaluated = R.evaluate(inputs, NOW)
        assert reason not in {f.reason for f in findings}, (reason, family)
        assert reason not in {key[0] for key in evaluated}, (reason, family)


def test_events_only_in_run_for_no_startup_probe():
    inputs = liveness_events()
    inputs.complete.discard("E")
    findings, evaluated = R.evaluate(inputs, NOW)
    assert "reliability.no_startup_probe" not in {k[0] for k in evaluated}, "a sweep review decides nothing"
    # ...except that a startupProbe added resolves the card without events.
    inputs = liveness_events(startup=True)
    inputs.complete.discard("E")
    assert "reliability.no_startup_probe" in {k[0] for k in R.evaluate(inputs, NOW)[1]}
    # Old events (beyond 60 min) do not count.
    inputs = liveness_events()
    inputs.events[0]["last"] = ts(hours=2)
    assert "reliability.no_startup_probe" not in _reasons(inputs)
    # Live (rl-oomhist): restarts that are out-of-memory kills are the oom incident, not a slow start.
    inputs = liveness_events()
    for p in next(iter(inputs.workloads.values())).pods:
        p["status"]["containerStatuses"][0]["lastState"] = {"terminated": {"reason": "OOMKilled", "exitCode": 137}}
    assert "reliability.no_startup_probe" not in _reasons(inputs)


def test_no_startup_probe_lands_on_the_namespace_with_the_symptom():
    twin = workload()
    twin["metadata"]["namespace"] = "shop-b"
    inputs = liveness_events()
    home = next(iter(inputs.workloads.values()))
    inputs = build([home.obj, twin], events=inputs.events, namespaces=[NS, "shop-b"],
                   pod_lists=[home.pods, pods(twin, restarts=2)])
    # Same workload, same pod name, both restarting: only the event's namespace has the symptom.
    found = [f for f in R.evaluate(inputs, NOW)[0] if f.reason == "reliability.no_startup_probe"]
    assert [(f.namespace, f.subject) for f in found] == [(NS, "deployment/web")]


def test_oom_history_reads_the_incident_of_its_namespace():
    inputs = build(doc=oom_doc())
    assert "resources.oom_history" in _reasons(inputs)
    for card in inputs.doc.insights:
        card.params["namespace"] = "shop-b"
    assert "resources.oom_history" not in _reasons(inputs), "the same workload's OOM in another namespace"


def _one(inputs, reason):
    return next(f for f in R.evaluate(inputs, NOW)[0] if f.reason == reason)


def test_production_boost_severity():
    assert _one(build([workload(replicas=1)]), "reliability.single_replica").severity == "info"
    prod_single = prod()
    prod_single.workloads[("shop-prod", SUBJECT)].obj["spec"]["replicas"] = 1
    assert _one(prod_single, "reliability.single_replica").severity == "warning"
    assert _one(prod(), "protection.production_uncovered").params["namespaces"] == "shop-prod"
    # The evidence names the namespaces that made the app production, not every namespace of the app.
    two = build([workload()], namespaces=[NS, "shop-prod"])
    assert [(e.type, e.ref) for e in _one(two, "protection.production_uncovered").evidence] == [
        ("object", "object:Namespace/shop-prod")]
    # A covering plan's environment named like production makes a non-prod namespace production too.
    envd = prod([plan(environmentID="e1", mode="audit")], ns="shop", envs={"e1": "Production"})
    assert "protection.production_audit_only" in {f.reason for f in R.evaluate(envd, NOW)[0]}
    # An HPA minimum decides the effective replica count.
    assert "reliability.single_replica" not in _reasons(build([workload(replicas=1)], hpas=[hpa(min_replicas=2)]))
    assert _one(build([workload(replicas=3)], hpas=[hpa(min_replicas=1)]), "reliability.single_replica").params[
        "hpa"] == "web"


def test_suggestions_rounded_and_floored():
    f = _one(used(10, 60 * MI), "resources.memory_near_limit")
    assert f.params["limit"] == "64Mi" and f.params["usage"] == "60Mi" and f.params["samples"] == "13"
    assert f.params["suggested"] == "80Mi" and f.key == "deployment/web#app/memory"
    f = _one(used(10, 20 * MI, resources=BIG), "resources.overprovisioned")
    assert f.params["suggested"] in ("16Mi", "32Mi", "10m", "20m")
    over = {g.key: g.params["suggested"] for g in R.evaluate(used(10, 20 * MI, resources=BIG), NOW)[0]
            if g.reason == "resources.overprovisioned"}
    assert over == {"deployment/web#app/cpu": "20m", "deployment/web#app/memory": "32Mi"}
    for p95, step in ((7, 10), (101, 10), (17 * MI, 16 * MI)):
        resource = "cpu" if step == 10 else "memory"
        value = R._suggest(p95, 1.2, resource)
        assert value % step == 0 and value >= p95 * 1.1
    under = {g.key: g.severity for g in R.evaluate(used(95, 60 * MI), NOW)[0] if g.reason == "resources.underprovisioned"}
    assert under == {"deployment/web#app/cpu": "info", "deployment/web#app/memory": "warning"}


def test_usage_gaps_and_short_history():
    # A sample that missed the container (a restart) is skipped; the rest still spans the window.
    inputs = used(10, 60 * MI, n=14)
    inputs.usage[(NS, SUBJECT)][3]["containers"] = {}
    assert _one(inputs, "resources.memory_near_limit").params["samples"] == "13"
    # Too few samples measured it: not judged at all (an existing card stays as it is).
    inputs = used(10, 60 * MI)
    for sample in inputs.usage[(NS, SUBJECT)][:5]:
        sample["containers"] = {}
    findings, evaluated = R.evaluate(inputs, NOW)
    assert not {"resources.memory_near_limit", "resources.underprovisioned"} & {k[0] for k in evaluated}


def test_workload_outside_the_listed_namespaces_is_not_judged_by_list_rules():
    """The review lists at most EVENT_NAMESPACES_MAX namespaces: elsewhere 'no PDB' only means 'not read'."""
    far = workload("far")
    far["metadata"]["namespace"] = "ns6"
    inputs = build([workload(), far])
    evaluated = {k for k in R.evaluate(inputs, NOW)[1] if k[2] == "deployment/far"}
    reasons = {k[0] for k in evaluated}
    assert not reasons & {"reliability.no_pdb", "networking.no_network_policy", "reliability.no_readiness_probe",
                          "reliability.single_replica"}
    assert "security.privileged" in reasons, "the workload itself was read: workload-only rules still judge it"


SECRET = "hunter2-lab"


def test_secret_value_never_in_output():
    inputs = env([{"name": "DB_PASSWORD", "value": SECRET}, {"name": "API-KEY", "value": SECRET + "x"}])
    findings, _ = R.evaluate(inputs, NOW)
    f = next(g for g in findings if g.reason == "security.plaintext_secret_env")
    assert f.params["envVars"] == "DB_PASSWORD, API-KEY"
    rendered = messages.render_recommendation(f.reason, f.params)
    assert SECRET not in repr(findings) + repr(rendered)


SECRET_NAMES = [
    ("DB_PASSWORD", "x1", True), ("PASSWD", "x1", True), ("GITHUB_TOKEN", "ghp_x", True), ("STRIPE_API_KEY", "sk", True),
    ("AWS_ACCESS_KEY", "AKIA", True), ("TLS_PRIVATE_KEY", "-----", True), ("APP_CREDENTIALS", "c", True),
    ("api-key", "v", True), ("DB_PASSWORD_FILE", "/run/secrets/db", False), ("JWT_SECRET_NAME", "my-secret", False),
    ("TOKEN_ENDPOINT", "https://x", False), ("TOKEN_TTL", "3600", False), ("SECRET_ENABLED", "true", False),
    ("DB_PASSWORD", "$(DB_PASS_FROM_SECRET)", False), ("DB_PASSWORD", "", False), ("DB_PASSWORD", "12345", False),
    ("DB_PASSWORD", "true", False), ("PASSWORDLESS", "yes-please", False), ("TOKENIZER_MODEL", "bpe", False),
    ("DB_PASSWORD", None, False),
]


@pytest.mark.parametrize("name,value,expected", SECRET_NAMES)
def test_secret_looking_names(name, value, expected):
    assert R.secret_looking(name, value) is expected


def test_privileged_subsumes_escalation():
    reasons = _reasons(sc_without("allowPrivilegeEscalation", privileged=True, capabilities={"add": ["SYS_ADMIN"]}))
    assert "security.privileged" in reasons
    assert not {"security.privilege_escalation_allowed", "security.added_capabilities",
                "security.capabilities_not_dropped", "security.seccomp_unset"} & reasons
    assert _one(sc_with(allowPrivilegeEscalation=True), "security.privilege_escalation_allowed").severity == "warning"
    assert _one(sc_without("allowPrivilegeEscalation"), "security.privilege_escalation_allowed").severity == "info"


def test_daemonset_severity_downgrades():
    def ds(**pod_over):
        return build([workload(kind="DaemonSet", containers=[container(securityContext={
            **{k: v for k, v in container()["securityContext"].items() if k != "allowPrivilegeEscalation"},
            "privileged": True}, ports=[{"name": "http", "containerPort": 8080, "hostPort": 9100}])], **pod_over)])

    inputs = ds(hostPID=True, volumes=[{"name": "host", "hostPath": {"path": "/proc"}}])
    severities = {f.reason: f.severity for f in R.evaluate(inputs, NOW)[0]}
    assert severities["security.privileged"] == "warning"
    assert severities["security.host_namespaces"] == "info" and severities["security.host_path"] == "info"
    assert severities["security.host_port"] == "info"
    # DaemonSets are skipped by the replica rules.
    assert not {"reliability.single_replica", "reliability.no_pdb"} & set(severities)
    host = _one(build([workload(hostNetwork=True, hostIPC=True)]), "security.host_namespaces")
    assert host.params["namespaces"] == "network and IPC namespaces" and host.severity == "warning"


def test_protection_skipped_when_plans_unavailable():
    inputs = prod()
    inputs.complete.discard("X")
    findings, evaluated = R.evaluate(inputs, NOW)
    assert not {k[0] for k in evaluated} & {"protection.production_uncovered", "protection.production_audit_only"}


def test_protection_coverage():
    workloads = [("shop-prod", "deployment", "web")]
    ns_plan = plan(scope={"type": "namespaces", "namespaces": ["shop-prod"]})
    assert R.covering_plans([ns_plan], "web", ["shop-prod"], workloads) == [ns_plan]
    excluded = plan(scope={"type": "applications", "applicationIds": ["web"], "exclusions": {"kinds": ["Deployment"]}})
    assert R.covering_plans([excluded], "web", ["shop-prod"], workloads) == []
    by_name = plan(scope={"type": "applications", "applicationIds": ["web"], "exclusions": {
        "resources": [{"kind": "Deployment", "name": "web", "namespace": "shop-prod"}]}})
    assert R.covering_plans([by_name], "web", ["shop-prod"], workloads) == []
    for other in (plan(phase="terminated"), plan(scope={"type": "applications", "applicationIds": ["api"]}),
                  plan(scope={"type": "bogus"})):
        assert R.covering_plans([other], "web", ["shop-prod"], workloads) == []
    assert "protection.production_uncovered" in _reasons(prod([plan(phase="draft")]))


def test_rollout_percent_and_int():
    def rolling(max_unavailable, replicas=2):
        obj = workload(replicas=replicas)
        obj["spec"]["strategy"] = {"type": "RollingUpdate", "rollingUpdate": {"maxUnavailable": max_unavailable}}
        return build([obj])

    assert _one(rolling("100%"), "reliability.rollout_all_at_once").params["maxUnavailable"] == "100%"
    assert "reliability.rollout_all_at_once" in _reasons(rolling(2))
    assert "reliability.rollout_all_at_once" not in _reasons(rolling("50%"))
    assert "reliability.rollout_all_at_once" not in _reasons(rolling("bogus"))
    assert "reliability.rollout_all_at_once" not in _reasons(strategy("Recreate") if False else rolling(1, 3))


def test_same_node_needs_two_running_pods():
    obj = workload(topologySpreadConstraints=[])
    inputs = build([obj], pod_lists=[pods(obj, count=1, nodes=("n1",))])
    assert "reliability.replicas_same_node" not in {k[0] for k in R.evaluate(inputs, NOW)[1]}
    anti = workload(topologySpreadConstraints=[], affinity={"podAntiAffinity": {"preferred": []}})
    assert "reliability.replicas_same_node" not in _reasons(build([anti], pod_lists=[pods(anti, nodes=("n1",))]))


def test_capped_workloads_skip_cross_workload_rules():
    inputs = selector_case(0)
    inputs.capped = True
    assert "networking.service_selector_mismatch" not in {k[0] for k in R.evaluate(inputs, NOW)[1]}
    inputs = skew(IMAGE.replace("a" * 64, "b" * 64))
    inputs.capped = True
    assert "consistency.image_skew" not in {k[0] for k in R.evaluate(inputs, NOW)[1]}


def test_selector_mismatch_unknown_pods_not_evaluated():
    inputs = selector_case(None)
    assert ("networking.service_selector_mismatch", NS, "service/web-a") not in R.evaluate(inputs, NOW)[1]
    external = build(services=[service("web"), service("ext", selector={"a": "b"}, type="ExternalName")],
                     extra_resources=[{"namespace": NS, "kind": "Service", "name": "ext"}])
    assert "networking.service_selector_mismatch" not in _reasons(external)


def test_numeric_target_port_mismatch_is_info():
    inputs = build(services=[service(target=9090)])
    f = _one(inputs, "networking.service_port_mismatch")
    assert (f.severity, f.confidence, f.params["ports"], f.subject) == ("info", "medium", "http", "service/web")


def test_image_skew_one_card_for_both_namespaces():
    findings, _ = R.evaluate(skew(IMAGE.replace("a" * 64, "b" * 64)), NOW)
    (f,) = [g for g in findings if g.reason == "consistency.image_skew"]
    assert (f.namespace, f.params["namespaces"]) == (NS, "shop, shop-b")


def test_gone_subjects_are_evaluated():
    """A card whose workload, Service or container is gone from a complete read resolves."""
    doc = AppInsights(insights=[
        Insight(id="a", kind="reliability", subject="deployment/old", status="open", category="recommendation",
                reason="reliability.no_liveness_probe", params={"namespace": NS}),
        Insight(id="b", kind="networking", subject="service/gone", status="updated", category="recommendation",
                reason="networking.service_port_mismatch", params={"namespace": NS}),
        Insight(id="c", kind="resources", subject=SUBJECT, status="open", category="recommendation",
                reason="resources.memory_near_limit", params={"namespace": NS, "container": "old", "resource": "memory"}),
        Insight(id="d", kind="change_risk", subject="application/web", status="open", category="recommendation",
                reason="change_risk.high_velocity", params={"namespace": NS}),
        Insight(id="e", kind="reliability", subject="deployment/web", status="resolved", category="recommendation",
                reason="reliability.no_pdb", params={"namespace": NS}),
    ])
    evaluated = R.evaluate(build(doc=doc), NOW)[1]
    assert ("reliability.no_liveness_probe", NS, "deployment/old") in evaluated
    assert ("networking.service_port_mismatch", NS, "service/gone") in evaluated
    assert ("resources.memory_near_limit", NS, "deployment/web#old/memory") in evaluated
    assert R.card_key("resources.memory_near_limit", SUBJECT, {"container": "app", "resource": "cpu"}) == (
        "deployment/web#app/cpu")


def test_cards_of_a_namespace_excluded_later_resolve():
    hidden = [{"namespace": "shop-b", "kind": "Deployment", "name": "old"},
              {"namespace": "shop-b", "kind": "Service", "name": "old"}]
    doc = AppInsights(insights=[
        Insight(id="a", kind="reliability", subject="deployment/old", status="open", category="recommendation",
                reason="reliability.no_liveness_probe", params={"namespace": "shop-b"}),
        Insight(id="b", kind="networking", subject="service/old", status="open", category="recommendation",
                reason="networking.service_port_mismatch", params={"namespace": "shop-b"}),
    ])
    keys = {("reliability.no_liveness_probe", "shop-b", "deployment/old"),
            ("networking.service_port_mismatch", "shop-b", "service/old")}
    inputs = build(doc=doc, extra_resources=hidden)
    assert not keys & R.evaluate(inputs, NOW)[1], "while the namespace is shown, an unread workload is no verdict"
    inputs.excluded = ["shop-b"]
    findings, evaluated = R.evaluate(inputs, NOW)
    assert keys <= evaluated
    stats = insights.merge_recommendations(doc, findings, evaluated, "2026-09-24T12:00:00Z", NS, "web")
    assert set(stats.resolved) == {"a", "b"}


def test_render_recommendation_modes_and_caps():
    title, summary = messages.render_recommendation("security.runs_as_root", {
        "workload": "web", "containers": "app", "mode": "unverified"})
    assert summary == "Container app of web does not enforce a non-root user; it runs as root if the image does."
    title, summary = messages.render_recommendation("reliability.single_replica", {"workload": "w" * 200})
    assert len(title) <= 120 and len(summary) <= 400


def test_findings_most_severe_first():
    inputs = sc_without("allowPrivilegeEscalation", "readOnlyRootFilesystem", privileged=True)
    severities = [f.severity for f in R.evaluate(inputs, NOW)[0]]
    ranks = {"critical": 2, "warning": 1, "info": 0}
    assert severities == sorted(severities, key=lambda s: -ranks[s]) and severities[0] == "critical"


def test_complete_set_constant():
    assert ALL == {"W", "S", "P", "H", "N", "U", "A", "D", "X"}
    assert copy.deepcopy(hardened()).complete == ALL
