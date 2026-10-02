"""The 60 deterministic v1 recommendation rules. Pure: reads review.ReviewInputs, calls nothing.

Each rule declares the input families it needs; it runs only when all of them are
complete, and returns the keys it evaluated with a finding or None. A key that was
evaluated and not found resolves its card; a key that was not evaluated (an unread
workload, too few usage samples, an unhealthy PDB) leaves its card untouched.
Container-level findings are aggregated per workload in `params.containers`, except
the four usage rules, which state one container's numbers (one card per container
and resource). Secret values are read only to test their shape and never copied.
"""

from __future__ import annotations

import math
import re
from collections.abc import Callable, Iterator
from dataclasses import dataclass, field
from datetime import datetime, timedelta

import messages
from config import (
    ANALYZER_CHANGE_RISK_MIN_SPAN_SEC,
    ANALYZER_CHANGE_VELOCITY_PER_DAY,
    ANALYZER_PRODUCTION_PATTERN,
)
from constants import (
    AND_SEPARATOR,
    CAPABILITY_ALL,
    CONDITION_FALSE_STATUS,
    CONDITION_SCALING_ACTIVE,
    CONFIG_VOLUME_SOURCES,
    K8S_KIND_NETWORK_POLICY,
    POLICY_TYPE_INGRESS,
    PROBE_KINDS,
    PROBE_PORT_HANDLERS,
    PROBE_SUFFIX,
    PROC_MOUNT_UNMASKED,
    PULL_POLICY_NEVER,
    REASON_SCALING_DISABLED,
    SECCOMP_UNCONFINED,
    SELECT_POLICY_DISABLED,
    CAPABILITY_PREFIX,
    CHANGE_RATE_WINDOW_DAYS,
    CHANGE_RATE_WINDOW_S,
    CONDITION_SCALING_LIMITED,
    CONDITION_TRUE,
    CONFIDENCE_HIGH,
    CONFIDENCE_MEDIUM,
    CPU_STEP_M,
    DANGEROUS_CAPABILITIES,
    DATE_FORMAT,
    DEFAULT_MAX_UNAVAILABLE,
    DEFAULT_SERVICE_ACCOUNT,
    ENV_FROM_TEMPLATE,
    ENV_NAME_SPLIT_PATTERN,
    ENV_REFERENCE_PATTERN,
    ENV_SCALAR_PATTERN,
    ENV_VARS_MAX,
    EVIDENCE_TYPE_CHANGE,
    EVIDENCE_TYPE_EVENT,
    EVIDENCE_TYPE_METRIC,
    EVIDENCE_TYPE_OBJECT,
    EVIDENCE_TYPE_PLAN,
    EVIDENCE_TYPE_SNAPSHOT,
    EVIDENCE_TYPE_SPEC,
    FAMILY_APP,
    FAMILY_DOCUMENT,
    FAMILY_EVENTS,
    FAMILY_HPAS,
    FAMILY_NETPOLS,
    FAMILY_PDBS,
    FAMILY_PLANS,
    FAMILY_SERVICES,
    FAMILY_USAGE,
    FAMILY_WORKLOADS,
    FREQUENT_ROLLBACKS_MIN,
    FULL_PERCENT,
    HOST_NAMESPACE_FIELDS,
    HOST_NAMESPACES_PLURAL_TEMPLATE,
    HOST_NAMESPACES_TEMPLATE,
    HOSTNAME_TOPOLOGY_KEY,
    HPA_METRIC_RESOURCE,
    HPA_TARGET_UTILIZATION,
    IMAGE_TAG_LATEST,
    INSIGHT_CATEGORY_RECOMMENDATION,
    INSIGHT_KIND_OOM,
    INSIGHT_SEVERITY_CRITICAL,
    INSIGHT_SEVERITY_INFO,
    INSIGHT_SEVERITY_WARNING,
    INSIGHT_STATUS_OPEN,
    INSIGHT_STATUS_RESOLVED,
    INSIGHT_STATUS_UPDATED,
    K8S_KIND_HPA,
    K8S_KIND_NAMESPACE,
    K8S_KIND_PDB,
    K8S_KIND_POD,
    K8S_KIND_SERVICE,
    KEY_TEMPLATE,
    LIMIT_HEADROOM,
    LIST_SEPARATOR,
    MAX_EVIDENCE_PER_INSIGHT,
    MAX_INSIGHT_PARAM_LENGTH,
    MAX_INSIGHT_PARAMS,
    MEM_STEP_B,
    MOVING_TAG_POLICIES,
    NAMESPACE_LISTS,
    NEAR_LIMIT_RATIO,
    NO_STARTUP_WINDOW_S,
    OOM_HISTORY_WINDOW_S,
    OVERPROVISION_MIN_CPU_M,
    OVERPROVISION_MIN_MEM_B,
    OVERPROVISION_RATIO,
    PARAM_ELLIPSIS,
    PERCENT_SUFFIX,
    PLAN_MODE_AUDIT,
    PLAN_PHASE_ACTIVE,
    PLAN_PHASE_SCHEDULED,
    PLAN_SCOPE_APPLICATIONS,
    PLAN_SCOPE_NAMESPACES,
    POD_PHASE_RUNNING,
    PROBE_LIVENESS,
    REASON_OOM_KILLED,
    REASON_SEPARATOR,
    REASON_TOO_MANY_REPLICAS,
    REASON_UNHEALTHY,
    RECOMMENDATION_CPU_NEAR_LIMIT,
    RECOMMENDATION_MEMORY_NEAR_LIMIT,
    RECOMMENDATION_OVERPROVISIONED,
    RECOMMENDATION_UNDERPROVISIONED,
    RECOMMENDATION_WORKLOAD_KINDS,
    REF_EVENT,
    REF_GENERATION,
    REF_METRIC,
    REF_METRIC_WORKLOAD,
    REF_OBJECT,
    REF_PLAN,
    REF_SNAPSHOT,
    REF_SPEC,
    REQUEST_HEADROOM,
    RESOURCE_CPU,
    RESOURCE_MEMORY,
    RFC3339_FORMAT,
    ROLLBACKS_WINDOW_S,
    ROOT_MODE_EXPLICIT,
    ROOT_MODE_UNVERIFIED,
    SECRET_NAME_SEGMENTS,
    SECRET_NAME_SKIP_SUFFIXES,
    SECRET_NAME_SUFFIXES,
    SELECTOR_PAIR_TEMPLATE,
    SELECTOR_SEPARATOR,
    SERVICE_TYPE_EXTERNAL_NAME,
    SEVERITY_RANK,
    SHORT_GRACE_MAX_S,
    STRATEGY_RECREATE,
    STRATEGY_ROLLING_UPDATE,
    SUBJECT_APPLICATION_TEMPLATE,
    SUBJECT_SERVICE_TEMPLATE,
    SUGGESTION_FLOOR,
    SUSTAINED_LOAD_RATIO,
    UNDERPROVISION_RATIO,
    USAGE_CPU_MILLI,
    USAGE_MEM_BYTES,
    USAGE_PERCENTILE,
    USAGE_SAMPLE_CONTAINERS,
    VELOCITY_FORMAT,
    WORKLOAD_DAEMONSET,
    WORKLOAD_DEPLOYMENT,
    WORKLOAD_KINDS,
)
from helpers import (
    format_cpu,
    format_mem,
    image_parts,
    is_production,
    iso_epoch,
    parse_cpu_milli,
    parse_mem_bytes,
    probe_signature,
    round_up,
    selector_matches,
)
from models import EvidenceRef
from review import ReviewInputs, WorkloadInput, sufficient
from tools.k8s_tools import owned_event, workload_matchers


@dataclass
class Finding:
    reason: str
    kind: str
    subject: str
    # The card identity: the subject, or '<subject>#<container>/<resource>' for the four usage rules.
    key: str
    namespace: str
    severity: str
    confidence: str
    evidence: list[EvidenceRef]
    params: dict[str, str]


@dataclass(frozen=True)
class Settings:
    velocity_per_day: float = ANALYZER_CHANGE_VELOCITY_PER_DAY
    change_risk_min_span_s: int = ANALYZER_CHANGE_RISK_MIN_SPAN_SEC
    production_pattern: re.Pattern = ANALYZER_PRODUCTION_PATTERN


NAMESPACE_FAMILIES = frozenset(NAMESPACE_LISTS)
# One evaluated key and its finding (None = evaluated, nothing found).
Result = list[tuple[str, "Finding | None"]]
SCOPE_WORKLOAD = "workload"
SCOPE_SERVICE = "service"
SCOPE_APP = "app"


@dataclass(frozen=True)
class Rule:
    families: frozenset[str]
    scope: str
    fn: Callable


@dataclass
class _Ctx:
    inputs: ReviewInputs
    now: datetime
    settings: Settings
    app_name: str = ""
    primary: str = ""
    production: bool = False
    covering: list[dict] = field(default_factory=list)


# ---- small readers -----------------------------------------------------------------------------------------
def _spec(wl: WorkloadInput) -> dict:
    return wl.obj.get("spec") or {}


def _pod_spec(wl: WorkloadInput) -> dict:
    return (_spec(wl).get("template") or {}).get("spec") or {}


def _labels(wl: WorkloadInput) -> dict:
    return ((_spec(wl).get("template") or {}).get("metadata") or {}).get("labels") or {}


def _containers(wl: WorkloadInput) -> list[dict]:
    return _pod_spec(wl).get("containers") or []


def _all_containers(wl: WorkloadInput) -> list[dict]:
    """Security rules judge init containers too; injected sidecars (in pods only) are never judged."""
    return (_pod_spec(wl).get("initContainers") or []) + _containers(wl)


def _replicas(wl: WorkloadInput) -> int | None:
    return None if wl.kind == WORKLOAD_DAEMONSET else _spec(wl).get("replicas", 1)


def _running(wl: WorkloadInput) -> list[dict]:
    return [p for p in wl.pods if (p.get("status") or {}).get("phase") == POD_PHASE_RUNNING]


def _resources(wl: WorkloadInput, name: str) -> dict:
    """The container's effective (admission-mutated) resources from a pod, Running pods first; else the template."""
    for pod in _running(wl) + wl.pods:
        for c in (pod.get("spec") or {}).get("containers") or []:
            if c.get("name") == name:
                return c.get("resources") or {}
    return next((c.get("resources") or {} for c in _containers(wl) if c.get("name") == name), {})


def _sc(container: dict, wl: WorkloadInput, key: str):
    """A securityContext field, the container's over the pod's."""
    own = container.get("securityContext") or {}
    return own[key] if key in own else (_pod_spec(wl).get("securityContext") or {}).get(key)


def _join(items) -> str:
    return LIST_SEPARATOR.join(dict.fromkeys(str(i) for i in items))


def _spec_ref(wl: WorkloadInput, path: str) -> EvidenceRef:
    return EvidenceRef(type=EVIDENCE_TYPE_SPEC, ref=REF_SPEC.format(subject=wl.subject, path=path))


def _object_ref(kind: str, name: str) -> EvidenceRef:
    return EvidenceRef(type=EVIDENCE_TYPE_OBJECT, ref=REF_OBJECT.format(kind=kind, name=name))


def _container_refs(wl: WorkloadInput, names: list[str], suffix: str) -> list[EvidenceRef]:
    return [_spec_ref(wl, f"containers/{n}/{suffix}") for n in names]


def _bound(params: dict) -> dict[str, str]:
    out = {}
    for key, value in params.items():
        if value is None or value == "":
            continue
        text = str(value)
        if len(text) > MAX_INSIGHT_PARAM_LENGTH:
            text = text[:MAX_INSIGHT_PARAM_LENGTH - len(PARAM_ELLIPSIS)] + PARAM_ELLIPSIS
        out[key] = text
    return dict(list(out.items())[:MAX_INSIGHT_PARAMS])


def _finding(reason: str, subject: str, namespace: str, severity: str, confidence: str, evidence: list[EvidenceRef],
             params: dict, key: str = "") -> Finding:
    return Finding(reason=reason, kind=reason.partition(REASON_SEPARATOR)[0], subject=subject, key=key or subject,
                   namespace=namespace, severity=severity, confidence=confidence,
                   evidence=evidence[:MAX_EVIDENCE_PER_INSIGHT], params=_bound(params))


def _wl_finding(reason: str, wl: WorkloadInput, severity: str, confidence: str, evidence: list[EvidenceRef],
                **params) -> Result:
    params = {"workload": wl.name, "namespace": wl.namespace, **params}
    return [(wl.subject, _finding(reason, wl.subject, wl.namespace, severity, confidence, evidence, params))]


def _none(wl: WorkloadInput) -> Result:
    return [(wl.subject, None)]


def _hpa(ctx: _Ctx, wl: WorkloadInput) -> dict | None:
    for hpa in ctx.inputs.items(FAMILY_HPAS, wl.namespace) or []:
        ref = (hpa.get("spec") or {}).get("scaleTargetRef") or {}
        if (ref.get("kind") or "").lower() == wl.kind and ref.get("name") == wl.name:
            return hpa
    return None


def _effective_min(ctx: _Ctx, wl: WorkloadInput) -> tuple[int, dict | None]:
    """The fewest replicas the workload may run: its autoscaler's minimum, else spec.replicas (absent = 1)."""
    hpa = _hpa(ctx, wl)
    if hpa is not None and _replicas(wl) != 0:
        return (hpa.get("spec") or {}).get("minReplicas", 1), hpa
    return _replicas(wl), None


def _name(obj: dict) -> str:
    return (obj.get("metadata") or {}).get("name") or ""


def _wl_production(ctx: _Ctx, wl: WorkloadInput) -> bool:
    """The workload's own namespace, or the environment of a covering plan whose scope reaches that namespace."""
    envs = [(ctx.inputs.env_names or {}).get(p.get("environmentRef"), "") for p in ctx.covering
            if (p.get("scope") or {}).get("type") != PLAN_SCOPE_NAMESPACES
            or wl.namespace in ((p.get("scope") or {}).get("namespaces") or [])]
    return is_production([wl.namespace], envs, ctx.settings.production_pattern)


def _prod_severity(ctx: _Ctx, wl: WorkloadInput) -> str:
    return INSIGHT_SEVERITY_WARNING if _wl_production(ctx, wl) else INSIGHT_SEVERITY_INFO


# ---- reliability ---------------------------------------------------------------------------------------------
def _single_replica(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if wl.kind not in RECOMMENDATION_WORKLOAD_KINDS or _replicas(wl) == 0:
        return _none(wl)
    minimum, hpa = _effective_min(ctx, wl)
    if minimum != 1:
        return _none(wl)
    refs = [_spec_ref(wl, "replicas")] + ([_object_ref(K8S_KIND_HPA, _name(hpa))] if hpa else [])
    return _wl_finding("reliability.single_replica", wl, _prod_severity(ctx, wl), CONFIDENCE_HIGH, refs,
                       kind=wl.kind, replicas=minimum, hpa=hpa and _name(hpa))


def _matching_pdbs(ctx: _Ctx, wl: WorkloadInput) -> list[dict]:
    return [p for p in ctx.inputs.items(FAMILY_PDBS, wl.namespace) or []
            if selector_matches((p.get("spec") or {}).get("selector"), _labels(wl))]


def _no_pdb(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if wl.kind not in RECOMMENDATION_WORKLOAD_KINDS:
        return _none(wl)
    minimum, _hpa_obj = _effective_min(ctx, wl)
    if (minimum or 0) < 2 or _matching_pdbs(ctx, wl):
        return _none(wl)
    return _wl_finding("reliability.no_pdb", wl, _prod_severity(ctx, wl), CONFIDENCE_HIGH, [_spec_ref(wl, "replicas")],
                       replicas=minimum)


def _pdb_blocks_eviction(ctx: _Ctx, wl: WorkloadInput) -> Result:
    pdbs = _matching_pdbs(ctx, wl)
    for pdb in pdbs:
        status, spec = pdb.get("status") or {}, pdb.get("spec") or {}
        if "disruptionsAllowed" not in status or not status.get("expectedPods"):
            return []
        if status.get("currentHealthy", 0) < status["expectedPods"]:
            # Unhealthy pods also make disruptionsAllowed 0: the budget itself cannot be judged now.
            return []
        if status["disruptionsAllowed"] == 0:
            return _wl_finding("reliability.pdb_blocks_eviction", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                               [_object_ref(K8S_KIND_PDB, _name(pdb))], pdb=_name(pdb),
                               minAvailable=spec.get("minAvailable"), maxUnavailable=spec.get("maxUnavailable"),
                               replicas=status["expectedPods"])
    return _none(wl)


def _declared_ports(container: dict) -> list[dict]:
    return container.get("ports") or []


def _port_matches(target, container: dict) -> bool:
    if isinstance(target, str):
        return any(p.get("name") == target for p in _declared_ports(container))
    return any(p.get("containerPort") == target for p in _declared_ports(container))


def _selecting_services(ctx: _Ctx, wl: WorkloadInput) -> list[dict]:
    """Services of the workload's namespace whose (non-empty) selector selects its pods."""
    out = []
    for svc in ctx.inputs.items(FAMILY_SERVICES, wl.namespace) or []:
        spec = svc.get("spec") or {}
        selector = spec.get("selector") or {}
        if selector and spec.get("type") != SERVICE_TYPE_EXTERNAL_NAME and selector_matches(
                {"matchLabels": selector}, _labels(wl)):
            out.append(svc)
    return out


def _targets(svc: dict) -> list:
    return [p.get("targetPort", p.get("port")) for p in (svc.get("spec") or {}).get("ports") or []]


def _no_readiness_probe(ctx: _Ctx, wl: WorkloadInput) -> Result:
    targets = [t for svc in _selecting_services(ctx, wl) for t in _targets(svc)]
    served = [c for c in _containers(wl) if any(_port_matches(t, c) for t in targets)]
    # Ports need not be declared: when none matches, a Service still reaches a container that declares none.
    if targets and not served:
        served = [c for c in _containers(wl) if not _declared_ports(c)]
    names = [c.get("name") for c in served if not c.get("readinessProbe")]
    if not names:
        return _none(wl)
    return _wl_finding("reliability.no_readiness_probe", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "readinessProbe"), containers=_join(names))


def _no_liveness_probe(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _containers(wl) if not c.get("livenessProbe")]
    if not names:
        return _none(wl)
    return _wl_finding("reliability.no_liveness_probe", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "livenessProbe"), containers=_join(names))


def _liveness_same_as_readiness(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _containers(wl)
             if (sig := probe_signature(c.get("livenessProbe"))) and sig == probe_signature(c.get("readinessProbe"))]
    if not names:
        return _none(wl)
    return _wl_finding("reliability.liveness_same_as_readiness", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "livenessProbe"), containers=_join(names))


def _restarts(wl: WorkloadInput) -> int:
    """Restarts a probe may explain: a container last killed for memory restarts for that reason (the oom
    incident), not because it starts slowly."""
    return sum(c.get("restartCount", 0) for p in wl.pods
               for c in ((p.get("status") or {}).get("containerStatuses") or [])
               if ((c.get("lastState") or {}).get("terminated") or {}).get("reason") != REASON_OOM_KILLED)


def _no_startup_probe(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _containers(wl) if c.get("livenessProbe") and not c.get("startupProbe")]
    if not names:
        return _none(wl)
    # Deciding needs this run's events: a sweep review leaves the card as it is.
    if FAMILY_EVENTS not in ctx.inputs.complete:
        return []
    since = (ctx.now - timedelta(seconds=NO_STARTUP_WINDOW_S)).strftime(RFC3339_FORMAT)
    matchers = workload_matchers(wl.kind, wl.name)
    events = [e for e in ctx.inputs.events if e.get("reason") == REASON_UNHEALTHY and e.get("last", "") >= since
              and messages.probe_of(e) == PROBE_LIVENESS and owned_event(e, wl.namespace, matchers)]
    restarts = _restarts(wl)
    if not events or restarts == 0:
        return _none(wl)
    refs = _container_refs(wl, names, "livenessProbe") + [
        EvidenceRef(type=EVIDENCE_TYPE_EVENT, ref=REF_EVENT.format(**events[0]))]
    return _wl_finding("reliability.no_startup_probe", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_MEDIUM, refs,
                       containers=_join(names), restarts=restarts)


def _pinned_to_one_host(pod_spec: dict) -> bool:
    if HOSTNAME_TOPOLOGY_KEY in (pod_spec.get("nodeSelector") or {}):
        return True
    required = (((pod_spec.get("affinity") or {}).get("nodeAffinity") or {})
                .get("requiredDuringSchedulingIgnoredDuringExecution") or {})
    return any(e.get("key") == HOSTNAME_TOPOLOGY_KEY and e.get("operator") == "In" and len(e.get("values") or []) == 1
               for term in required.get("nodeSelectorTerms") or [] for e in term.get("matchExpressions") or [])


def _replicas_same_node(ctx: _Ctx, wl: WorkloadInput) -> Result:
    replicas = _replicas(wl)
    pod_spec = _pod_spec(wl)
    spread = any(c.get("topologyKey") == HOSTNAME_TOPOLOGY_KEY for c in pod_spec.get("topologySpreadConstraints") or [])
    anti = bool((pod_spec.get("affinity") or {}).get("podAntiAffinity"))
    if replicas is None or replicas < 2 or spread or anti or _pinned_to_one_host(pod_spec):
        return _none(wl)
    running = _running(wl)
    if len(running) < 2:
        return []
    nodes = {(p.get("spec") or {}).get("nodeName") for p in running}
    if len(nodes) != 1 or None in nodes:
        return _none(wl)
    refs = [_spec_ref(wl, "affinity")] + [_object_ref(K8S_KIND_POD, _name(p)) for p in running[:2]]
    return _wl_finding("reliability.replicas_same_node", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, refs,
                       replicas=len(running), node=next(iter(nodes)))


def _percent_or_count(value, replicas: int) -> int | None:
    if isinstance(value, int):
        return value
    if isinstance(value, str) and value.endswith(PERCENT_SUFFIX) and value[:-1].isdigit():
        return math.floor(replicas * int(value[:-1]) / FULL_PERCENT)
    return None


def _rollout_all_at_once(ctx: _Ctx, wl: WorkloadInput) -> Result:
    replicas = _replicas(wl)
    if wl.kind != WORKLOAD_DEPLOYMENT or (replicas or 0) < 2:
        return _none(wl)
    strategy = _spec(wl).get("strategy") or {}
    kind = strategy.get("type") or STRATEGY_ROLLING_UPDATE
    pvc = any(v.get("persistentVolumeClaim") for v in _pod_spec(wl).get("volumes") or [])
    max_unavailable = (strategy.get("rollingUpdate") or {}).get("maxUnavailable", DEFAULT_MAX_UNAVAILABLE)
    if kind == STRATEGY_RECREATE:
        fires, max_unavailable = not pvc, ""
    else:
        fires = (_percent_or_count(max_unavailable, replicas) or 0) >= replicas
    if not fires:
        return _none(wl)
    return _wl_finding("reliability.rollout_all_at_once", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "strategy")], strategy=kind, maxUnavailable=max_unavailable, replicas=replicas)


def _short_grace_period(ctx: _Ctx, wl: WorkloadInput) -> Result:
    grace = _pod_spec(wl).get("terminationGracePeriodSeconds")
    if grace is None or grace > SHORT_GRACE_MAX_S:
        return _none(wl)
    return _wl_finding("reliability.short_grace_period", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "terminationGracePeriodSeconds")], grace=grace)


# ---- resources -----------------------------------------------------------------------------------------------
_RESOURCES = (RESOURCE_CPU, RESOURCE_MEMORY)


def _first_group(pairs: list[tuple[str, tuple[str, ...]]]) -> tuple[list[str], tuple[str, ...]]:
    """The containers sharing the first offender's set of resources, so every stated resource is true of each."""
    first = pairs[0][1]
    return [name for name, missing in pairs if missing == first], first


def _no_requests(ctx: _Ctx, wl: WorkloadInput) -> Result:
    pairs = []
    for c in _containers(wl):
        res = _resources(wl, c.get("name"))
        missing = tuple(r for r in _RESOURCES if r not in (res.get("requests") or {})
                        and r not in (res.get("limits") or {}))
        if missing:
            pairs.append((c.get("name"), missing))
    if not pairs:
        return _none(wl)
    names, missing = _first_group(pairs)
    qos = next(((p.get("status") or {}).get("qosClass") for p in _running(wl)), None)
    return _wl_finding("resources.no_requests", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "resources"), containers=_join(names),
                       missing=AND_SEPARATOR.join(missing), qos=qos)


def _no_memory_limit(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _containers(wl)
             if RESOURCE_MEMORY not in (_resources(wl, c.get("name")).get("limits") or {})]
    if not names:
        return _none(wl)
    return _wl_finding("resources.no_memory_limit", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "resources/limits"), containers=_join(names))


def _limits_without_requests(ctx: _Ctx, wl: WorkloadInput) -> Result:
    pairs = []
    for c in _containers(wl):
        res = c.get("resources") or {}
        only = tuple(r for r in _RESOURCES if r in (res.get("limits") or {}) and r not in (res.get("requests") or {}))
        if only:
            pairs.append((c.get("name"), only))
    if not pairs:
        return _none(wl)
    names, only = _first_group(pairs)
    return _wl_finding("resources.limits_without_requests", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "resources"), containers=_join(names),
                       resource=AND_SEPARATOR.join(only))


_USAGE = {  # resource -> (sample key, parse, format, suggestion step)
    RESOURCE_CPU: (USAGE_CPU_MILLI, parse_cpu_milli, format_cpu, CPU_STEP_M),
    RESOURCE_MEMORY: (USAGE_MEM_BYTES, parse_mem_bytes, format_mem, MEM_STEP_B),
}


def _p95(values: list[float]) -> float:
    ordered = sorted(values)
    return ordered[max(0, math.ceil(USAGE_PERCENTILE * len(ordered)) - 1)]


def _value(sample: dict, container: str, resource: str) -> float | None:
    return ((sample.get(USAGE_SAMPLE_CONTAINERS) or {}).get(container) or {}).get(_USAGE[resource][0])


def _series(ctx: _Ctx, wl: WorkloadInput, containers: list[str], resource: str) -> list[list[float]] | None:
    """Per container, its usage values over the samples that measured every one of them; None when those
    samples are too few or span too short a time to judge."""
    samples = [s for s in ctx.inputs.usage.get((wl.namespace, wl.subject)) or []
               if all(_value(s, c, resource) is not None for c in containers)]
    if not samples or not sufficient(samples):
        return None
    return [[_value(s, c, resource) for s in samples] for c in containers]


def _suggest(p95: float, headroom: float, resource: str) -> int:
    return round_up(max(p95 * headroom, p95 * SUGGESTION_FLOOR), _USAGE[resource][3])


def _usage_rule(ctx: _Ctx, wl: WorkloadInput, reason: str,
                judge: Callable[[str, float, dict], tuple[str, str, dict] | None],
                resources: tuple[str, ...] = _RESOURCES) -> Result:
    """One key per container and resource with enough history; judge(resource, p95, resources) names a finding."""
    out: Result = []
    for c in _containers(wl):
        name = c.get("name")
        for resource in resources:
            series = _series(ctx, wl, [name], resource)
            if series is None:
                continue
            values = series[0]
            key = KEY_TEMPLATE.format(subject=wl.subject, container=name, resource=resource)
            p95 = _p95(values)
            verdict = judge(resource, p95, _resources(wl, name))
            finding = None
            if verdict:
                severity, confidence, params = verdict
                ref = EvidenceRef(type=EVIDENCE_TYPE_METRIC, ref=REF_METRIC.format(subject=wl.subject, container=name,
                                                                                  resource=resource))
                fmt = _USAGE[resource][2]
                params = {"workload": wl.name, "namespace": wl.namespace, "container": name, "resource": resource,
                          "usage": fmt(math.ceil(p95)), "samples": len(values), **params}
                finding = _finding(reason, wl.subject, wl.namespace, severity, confidence, [ref], params, key)
            out.append((key, finding))
    return out


def _quantity(resources: dict, section: str, resource: str) -> int | None:
    return _USAGE[resource][1]((resources.get(section) or {}).get(resource))


def _near_limit(reason: str, resource: str, confidence: str):
    def judge(res: str, p95: float, resources: dict):
        limit = _quantity(resources, "limits", res)
        if not limit or p95 < NEAR_LIMIT_RATIO * limit:
            return None
        fmt = _USAGE[res][2]
        return INSIGHT_SEVERITY_WARNING, confidence, {"limit": fmt(limit),
                                                      "suggested": fmt(_suggest(p95, LIMIT_HEADROOM, res))}

    return lambda ctx, wl: _usage_rule(ctx, wl, reason, judge, (resource,))


def _overprovisioned(ctx: _Ctx, wl: WorkloadInput) -> Result:
    floors = {RESOURCE_CPU: OVERPROVISION_MIN_CPU_M, RESOURCE_MEMORY: OVERPROVISION_MIN_MEM_B}

    def judge(res: str, p95: float, resources: dict):
        request = _quantity(resources, "requests", res)
        if not request or request < floors[res] or p95 > OVERPROVISION_RATIO * request:
            return None
        fmt = _USAGE[res][2]
        return INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM, {"request": fmt(request),
                                                          "suggested": fmt(_suggest(p95, REQUEST_HEADROOM, res))}

    return _usage_rule(ctx, wl, RECOMMENDATION_OVERPROVISIONED, judge)


def _underprovisioned(ctx: _Ctx, wl: WorkloadInput) -> Result:
    def judge(res: str, p95: float, resources: dict):
        request = _quantity(resources, "requests", res)
        if not request or p95 <= UNDERPROVISION_RATIO * request:
            return None
        fmt = _USAGE[res][2]
        severity = INSIGHT_SEVERITY_WARNING if res == RESOURCE_MEMORY else INSIGHT_SEVERITY_INFO
        return severity, CONFIDENCE_HIGH, {"request": fmt(request),
                                           "suggested": fmt(_suggest(p95, REQUEST_HEADROOM, res))}

    return _usage_rule(ctx, wl, RECOMMENDATION_UNDERPROVISIONED, judge)


def _oom_history(ctx: _Ctx, wl: WorkloadInput) -> Result:
    # A card that names no namespace predates incident ids with one: it is taken, as before.
    cards = [c for c in ctx.inputs.doc.insights
             if c.category != INSIGHT_CATEGORY_RECOMMENDATION and c.kind == INSIGHT_KIND_OOM and c.subject == wl.subject
             and c.params.get("namespace", wl.namespace) == wl.namespace]
    if any(c.status != INSIGHT_STATUS_RESOLVED for c in cards):
        return []
    since = (ctx.now - timedelta(seconds=OOM_HISTORY_WINDOW_S)).strftime(RFC3339_FORMAT)
    for card in sorted(cards, key=lambda c: c.resolvedAt, reverse=True):
        container, limit = card.params.get("container"), card.params.get("limit")
        if card.resolvedAt < since or not (container and limit):
            continue
        current = (_resources(wl, container).get("limits") or {}).get(RESOURCE_MEMORY)
        if current is None or parse_mem_bytes(current) != parse_mem_bytes(limit):
            continue
        when = datetime.fromisoformat(card.lastSeenAt).strftime(DATE_FORMAT)
        return _wl_finding("resources.oom_history", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                           _container_refs(wl, [container], "resources/limits/memory"), container=container,
                           limit=current, lastOom=when)
    return _none(wl)


# ---- scaling -------------------------------------------------------------------------------------------------
def _hpa_bounds(hpa: dict) -> tuple[int, int]:
    spec = hpa.get("spec") or {}
    return spec.get("minReplicas", 1), spec.get("maxReplicas", 0)


def _hpa_min_equals_max(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    if hpa is None:
        return _none(wl)
    low, high = _hpa_bounds(hpa)
    if low != high:
        return _none(wl)
    return _wl_finding("scaling.hpa_min_equals_max", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_object_ref(K8S_KIND_HPA, _name(hpa))], hpa=_name(hpa), replicas=low)


def _hpa_missing_requests(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    if hpa is None:
        return _none(wl)
    for metric in (hpa.get("spec") or {}).get("metrics") or []:
        resource = metric.get("resource") or {}
        if metric.get("type") != HPA_METRIC_RESOURCE or (resource.get("target") or {}).get("type") != HPA_TARGET_UTILIZATION:
            continue
        name = resource.get("name")
        names = [c.get("name") for c in _containers(wl)
                 if name not in (_resources(wl, c.get("name")).get("requests") or {})]
        if names:
            refs = [_object_ref(K8S_KIND_HPA, _name(hpa))] + _container_refs(wl, names, "resources/requests")
            return _wl_finding("scaling.hpa_missing_requests", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, refs,
                               hpa=_name(hpa), resource=name, containers=_join(names))
    return _none(wl)


def _hpa_at_max(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    if hpa is None:
        return _none(wl)
    low, high = _hpa_bounds(hpa)
    status = hpa.get("status") or {}
    limited = any(c.get("type") == CONDITION_SCALING_LIMITED and c.get("status") == CONDITION_TRUE
                  and c.get("reason") == REASON_TOO_MANY_REPLICAS for c in status.get("conditions") or [])
    if low == high or not limited or status.get("currentReplicas") != high:
        return _none(wl)
    return _wl_finding("scaling.hpa_at_max", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       [_object_ref(K8S_KIND_HPA, _name(hpa))], hpa=_name(hpa), hpaMax=high)


def _no_hpa_sustained_load(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if wl.kind not in RECOMMENDATION_WORKLOAD_KINDS or _hpa(ctx, wl) is not None:
        return _none(wl)
    names = [c.get("name") for c in _containers(wl)]
    series = _series(ctx, wl, names, RESOURCE_CPU) if names else None
    if series is None:
        return []
    requests = [_quantity(_resources(wl, n), "requests", RESOURCE_CPU) for n in names]
    if not all(requests):
        return _none(wl)
    p95 = _p95([sum(values) for values in zip(*series)])
    request = sum(requests)
    if p95 < SUSTAINED_LOAD_RATIO * request:
        return _none(wl)
    ref = EvidenceRef(type=EVIDENCE_TYPE_METRIC, ref=REF_METRIC_WORKLOAD.format(subject=wl.subject,
                                                                                resource=RESOURCE_CPU))
    return _wl_finding("scaling.no_hpa_sustained_load", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM, [ref],
                       usage=format_cpu(math.ceil(p95)), request=format_cpu(request), samples=len(series[0]))


# ---- security --------------------------------------------------------------------------------------------------
def _is_privileged(c: dict) -> bool:
    return (c.get("securityContext") or {}).get("privileged") is True


def _node_agent(wl: WorkloadInput, severity: str) -> str:
    """DaemonSets are node agents: host access is usually their purpose, one level lower."""
    if wl.kind != WORKLOAD_DAEMONSET:
        return severity
    return INSIGHT_SEVERITY_WARNING if severity == INSIGHT_SEVERITY_CRITICAL else INSIGHT_SEVERITY_INFO


def _privileged(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl) if _is_privileged(c)]
    if not names:
        return _none(wl)
    return _wl_finding("security.privileged", wl, _node_agent(wl, INSIGHT_SEVERITY_CRITICAL), CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext/privileged"), containers=_join(names))


def _privilege_escalation(ctx: _Ctx, wl: WorkloadInput) -> Result:
    flagged = [(c.get("name"), (c.get("securityContext") or {}).get("allowPrivilegeEscalation"))
               for c in _all_containers(wl) if not _is_privileged(c)
               and (c.get("securityContext") or {}).get("allowPrivilegeEscalation") is not False]
    if not flagged:
        return _none(wl)
    explicit = any(value is True for _, value in flagged)
    severity = INSIGHT_SEVERITY_WARNING if explicit else INSIGHT_SEVERITY_INFO
    names = [n for n, _ in flagged]
    return _wl_finding("security.privilege_escalation_allowed", wl, severity, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext"), containers=_join(names))


def _runs_as_root(ctx: _Ctx, wl: WorkloadInput) -> Result:
    explicit, unverified = [], []
    for c in _all_containers(wl):
        user = _sc(c, wl, "runAsUser")
        if user == 0:
            explicit.append(c.get("name"))
        elif user is None and _sc(c, wl, "runAsNonRoot") is not True:
            unverified.append(c.get("name"))
    if not (explicit or unverified):
        return _none(wl)
    names, mode = (explicit, ROOT_MODE_EXPLICIT) if explicit else (unverified, ROOT_MODE_UNVERIFIED)
    severity, confidence = ((INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH) if explicit
                            else (INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM))
    return _wl_finding("security.runs_as_root", wl, severity, confidence,
                       _container_refs(wl, names, "securityContext"), containers=_join(names), mode=mode)


def _writable_root_fs(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl)
             if (c.get("securityContext") or {}).get("readOnlyRootFilesystem") is not True]
    if not names:
        return _none(wl)
    return _wl_finding("security.writable_root_fs", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext"), containers=_join(names))


def _capability(name: str) -> str:
    name = str(name).upper()
    return name[len(CAPABILITY_PREFIX):] if name.startswith(CAPABILITY_PREFIX) else name


def _added_capabilities(ctx: _Ctx, wl: WorkloadInput) -> Result:
    added = {c.get("name"): [_capability(a) for a in ((c.get("securityContext") or {}).get("capabilities") or {})
                             .get("add") or []]
             for c in _all_containers(wl) if not _is_privileged(c)}
    added = {n: caps for n, caps in added.items() if caps}
    if not added:
        return _none(wl)
    caps = [cap for values in added.values() for cap in values]
    severity = INSIGHT_SEVERITY_WARNING if set(caps) & set(DANGEROUS_CAPABILITIES) else INSIGHT_SEVERITY_INFO
    return _wl_finding("security.added_capabilities", wl, severity, CONFIDENCE_HIGH,
                       _container_refs(wl, list(added), "securityContext/capabilities"),
                       containers=_join(added), capabilities=_join(caps))


def _host_namespaces(ctx: _Ctx, wl: WorkloadInput) -> Result:
    shared = [label for key, label in HOST_NAMESPACE_FIELDS if _pod_spec(wl).get(key) is True]
    if not shared:
        return _none(wl)
    template = HOST_NAMESPACES_TEMPLATE if len(shared) == 1 else HOST_NAMESPACES_PLURAL_TEMPLATE
    fields = [key for key, label in HOST_NAMESPACE_FIELDS if label in shared]
    return _wl_finding("security.host_namespaces", wl, _node_agent(wl, INSIGHT_SEVERITY_WARNING), CONFIDENCE_HIGH,
                       [_spec_ref(wl, f) for f in fields], namespaces=template.format(AND_SEPARATOR.join(shared)))


def _host_path(ctx: _Ctx, wl: WorkloadInput) -> Result:
    volumes = [v.get("name") for v in _pod_spec(wl).get("volumes") or [] if v.get("hostPath")]
    if not volumes:
        return _none(wl)
    mounts = [m for c in _all_containers(wl) for m in c.get("volumeMounts") or [] if m.get("name") in volumes]
    read_only = all(m.get("readOnly") is True for m in mounts)
    severity = INSIGHT_SEVERITY_INFO if read_only or wl.kind == WORKLOAD_DAEMONSET else INSIGHT_SEVERITY_WARNING
    return _wl_finding("security.host_path", wl, severity, CONFIDENCE_HIGH,
                       [_spec_ref(wl, f"volumes/{v}") for v in volumes], volumes=_join(volumes))


def _default_service_account(ctx: _Ctx, wl: WorkloadInput) -> Result:
    pod_spec = _pod_spec(wl)
    account = pod_spec.get("serviceAccountName") or pod_spec.get("serviceAccount") or ""
    if account not in ("", DEFAULT_SERVICE_ACCOUNT):
        return _none(wl)
    return _wl_finding("security.default_service_account", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "serviceAccountName")])


def _api_token_mounted(pod: dict) -> bool:
    """The API-server token (kube-api-access-*): a projected serviceAccountToken without an audience.
    IRSA, EKS Pod Identity and Vault tokens set an audience and never count."""
    return any("serviceAccountToken" in source and not (source["serviceAccountToken"] or {}).get("audience")
               for v in (pod.get("spec") or {}).get("volumes") or []
               for source in (v.get("projected") or {}).get("sources") or [])


def _token_automount(ctx: _Ctx, wl: WorkloadInput) -> Result:
    running = _running(wl)
    if not running:
        return []
    if not any(_api_token_mounted(p) for p in running):
        return _none(wl)
    return _wl_finding("security.token_automount", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "automountServiceAccountToken")])


def _secrets_in_env(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names, env_vars = [], []
    for c in _all_containers(wl):
        found = [e.get("name") for e in c.get("env") or [] if ((e.get("valueFrom") or {}).get("secretKeyRef"))]
        found += [ENV_FROM_TEMPLATE.format((e.get("secretRef") or {}).get("name"))
                  for e in c.get("envFrom") or [] if e.get("secretRef")]
        if found:
            names.append(c.get("name"))
            env_vars += found
    if not names:
        return _none(wl)
    return _wl_finding("security.secrets_in_env", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "env"), containers=_join(names),
                       envVars=_join(list(dict.fromkeys(env_vars))[:ENV_VARS_MAX]))


def secret_looking(name: str, value) -> bool:
    """A literal env value whose name says it is a secret. The value is only tested for its shape."""
    upper = (name or "").upper().replace("-", "_")
    if upper.endswith(SECRET_NAME_SKIP_SUFFIXES):
        return False
    named = (any(seg in SECRET_NAME_SEGMENTS for seg in re.split(ENV_NAME_SPLIT_PATTERN, upper))
             or upper.endswith(SECRET_NAME_SUFFIXES))
    if not named or not isinstance(value, str) or not value.strip():
        return False
    return not (re.fullmatch(ENV_REFERENCE_PATTERN, value.strip())
                or re.fullmatch(ENV_SCALAR_PATTERN, value.strip(), re.IGNORECASE))


def _plaintext_secret_env(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names, env_vars = [], []
    for c in _all_containers(wl):
        found = [e.get("name") for e in c.get("env") or [] if "value" in e and secret_looking(e.get("name"),
                                                                                             e.get("value"))]
        if found:
            names.append(c.get("name"))
            env_vars += found
    if not names:
        return _none(wl)
    return _wl_finding("security.plaintext_secret_env", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_MEDIUM,
                       _container_refs(wl, names, "env"), containers=_join(names),
                       envVars=_join(list(dict.fromkeys(env_vars))[:ENV_VARS_MAX]))


# ---- images / config ---------------------------------------------------------------------------------------------
def _moving(image: str) -> bool:
    parts = image_parts(image)
    return parts is not None and not parts[3] and parts[2] in ("", IMAGE_TAG_LATEST)


def _mutable_tag(ctx: _Ctx, wl: WorkloadInput) -> Result:
    moving = [c for c in _containers(wl) if _moving(c.get("image") or "")]
    if not moving:
        return _none(wl)
    names = [c.get("name") for c in moving]
    return _wl_finding("images.mutable_tag", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "image"), containers=_join(names), image=moving[0].get("image"))


def _pull_policy_mismatch(ctx: _Ctx, wl: WorkloadInput) -> Result:
    stale = [c for c in _containers(wl) if _moving(c.get("image") or "")
             and c.get("imagePullPolicy") in MOVING_TAG_POLICIES]
    if not stale:
        return _none(wl)
    names = [c.get("name") for c in stale]
    return _wl_finding("images.pull_policy_mismatch", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "imagePullPolicy"), containers=_join(names),
                       image=stale[0].get("image"), pullPolicy=stale[0].get("imagePullPolicy"))


def _duplicate_env(ctx: _Ctx, wl: WorkloadInput) -> Result:
    for c in _containers(wl):
        seen = [e.get("name") for e in c.get("env") or []]
        dupes = [n for n in dict.fromkeys(seen) if seen.count(n) > 1]
        if dupes:
            return _wl_finding("config.duplicate_env", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                               _container_refs(wl, [c.get("name")], "env"), container=c.get("name"),
                               envVars=_join(dupes[:ENV_VARS_MAX]))
    return _none(wl)


# ---- networking ------------------------------------------------------------------------------------------------
def _app_services(ctx: _Ctx) -> list[tuple[str, dict]]:
    """(namespace, Service) of every Service the app groups, as the namespace list shows it now."""
    out = []
    for res in ctx.inputs.app.get("resources") or []:
        if (res.get("kind") or "").lower() != K8S_KIND_SERVICE.lower() or res.get("namespace") not in ctx.inputs.namespaces:
            continue
        svc = next((s for s in ctx.inputs.items(FAMILY_SERVICES, res.get("namespace")) or []
                    if _name(s) == res.get("name")), None)
        if svc is not None:
            out.append((res.get("namespace"), svc))
    return out


def _service_finding(reason: str, namespace: str, svc: dict, severity: str, confidence: str, **params) -> Finding:
    subject = SUBJECT_SERVICE_TEMPLATE.format(name=_name(svc))
    return _finding(reason, subject, namespace, severity, confidence, [_object_ref(K8S_KIND_SERVICE, _name(svc))],
                    {"service": _name(svc), "namespace": namespace, **params})


def _service_selector_mismatch(ctx: _Ctx, target: tuple[str, dict]) -> Result:
    namespace, svc = target
    subject = SUBJECT_SERVICE_TEMPLATE.format(name=_name(svc))
    spec = svc.get("spec") or {}
    selector = spec.get("selector") or {}
    if not selector or spec.get("type") == SERVICE_TYPE_EXTERNAL_NAME:
        return [(subject, None)]
    if ctx.inputs.capped:
        return []
    # A selector matching a workload's template is right even with 0 pods (scaled down, not created yet).
    if any(wl.namespace == namespace and selector_matches({"matchLabels": selector}, _labels(wl))
           for wl in ctx.inputs.workloads.values()):
        return [(subject, None)]
    pods = ctx.inputs.selector_pods.get((namespace, _name(svc)))
    if pods is None:
        return []
    if pods:
        return [(subject, None)]
    text = SELECTOR_SEPARATOR.join(SELECTOR_PAIR_TEMPLATE.format(k, v) for k, v in selector.items())
    return [(subject, _service_finding("networking.service_selector_mismatch", namespace, svc,
                                       INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, selector=text))]


def _service_port_mismatch(ctx: _Ctx, target: tuple[str, dict]) -> Result:
    namespace, svc = target
    subject = SUBJECT_SERVICE_TEMPLATE.format(name=_name(svc))
    for wl in ctx.inputs.workloads.values():
        if wl.namespace != namespace or svc not in _selecting_services(ctx, wl):
            continue
        containers = _containers(wl)
        declared = [p for c in containers for p in _declared_ports(c)]
        for target_port in _targets(svc):
            if any(_port_matches(target_port, c) for c in containers):
                continue
            named = isinstance(target_port, str)
            if not named and not declared:
                continue
            ports = _join(p.get("name") or p.get("containerPort") for p in declared) if declared else "no ports"
            severity, confidence = ((INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH) if named
                                    else (INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM))
            return [(subject, _service_finding("networking.service_port_mismatch", namespace, svc, severity,
                                               confidence, workload=wl.name, targetPort=target_port, ports=ports))]
    return [(subject, None)]


def _no_network_policy(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if any(selector_matches((p.get("spec") or {}).get("podSelector", {}), _labels(wl))
           for p in ctx.inputs.items(FAMILY_NETPOLS, wl.namespace) or []):
        return _none(wl)
    return _wl_finding("networking.no_network_policy", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM,
                       [_spec_ref(wl, "labels")])


# ---- app level -----------------------------------------------------------------------------------------------
def _app_subject(ctx: _Ctx) -> str:
    return SUBJECT_APPLICATION_TEMPLATE.format(name=ctx.app_name)


def _app_finding(ctx: _Ctx, reason: str, severity: str, confidence: str, evidence: list[EvidenceRef],
                 **params) -> Result:
    subject = _app_subject(ctx)
    return [(subject, _finding(reason, subject, ctx.primary, severity, confidence, evidence,
                               {"app": ctx.app_name, "namespace": ctx.primary, **params}))]


def _high_velocity(ctx: _Ctx, target) -> Result:
    now = ctx.now.timestamp()
    first = iso_epoch(((ctx.inputs.app.get("metrics") or {}).get("derived") or {}).get("firstChangeDetectedAt"))
    if first is None or now - first < ctx.settings.change_risk_min_span_s:
        return [(_app_subject(ctx), None)]
    log = (ctx.inputs.app.get("history") or {}).get("changeLog") or []
    recent = [e for e in log if e.get("changeClass") and (t := iso_epoch(e.get("detectedAt"))) is not None
              and t >= now - CHANGE_RATE_WINDOW_S]
    rate = len(recent) / CHANGE_RATE_WINDOW_DAYS
    if rate < ctx.settings.velocity_per_day:
        return [(_app_subject(ctx), None)]
    latest = max(recent, key=lambda e: e.get("generation", 0))
    ref = EvidenceRef(type=EVIDENCE_TYPE_CHANGE, ref=REF_GENERATION.format(generation=latest.get("generation")))
    return _app_finding(ctx, "change_risk.high_velocity", INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM, [ref],
                        velocity=VELOCITY_FORMAT.format(rate))


def _frequent_rollbacks(ctx: _Ctx, target) -> Result:
    now = ctx.now.timestamp()
    recent = [r for r in ctx.inputs.app.get("rollbacks") or []
              if (t := iso_epoch(r.get("triggeredAt"))) is not None and t >= now - ROLLBACKS_WINDOW_S]
    if len(recent) < FREQUENT_ROLLBACKS_MIN:
        return [(_app_subject(ctx), None)]
    latest = max(recent, key=lambda r: iso_epoch(r.get("triggeredAt")))
    ref = EvidenceRef(type=EVIDENCE_TYPE_SNAPSHOT, ref=REF_SNAPSHOT.format(id=latest.get("targetSnapshotId")))
    return _app_finding(ctx, "change_risk.frequent_rollbacks", INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, [ref],
                        rollbacks=len(recent))


def _excludes_every_workload(plan: dict, workloads: list[tuple[str, str, str]]) -> bool:
    exclusions = (plan.get("scope") or {}).get("exclusions") or {}
    kinds = {k.lower() for k in exclusions.get("kinds") or []}
    resources = {((r.get("kind") or "").lower(), r.get("name"), r.get("namespace"))
                 for r in exclusions.get("resources") or []}
    return bool(workloads) and all(kind in kinds or (kind, name, ns) in resources for ns, kind, name in workloads)


def covering_plans(plans: list[dict], app_name: str, namespaces: list[str], workloads: list[tuple[str, str, str]],
                   phases: tuple[str, ...] = (PLAN_PHASE_ACTIVE, PLAN_PHASE_SCHEDULED)) -> list[dict]:
    """Plans in `phases` whose scope names the app or one of its namespaces, minus those excluding every workload."""
    out = []
    for plan in plans:
        scope = plan.get("scope") or {}
        if plan.get("phase") not in phases:
            continue
        if scope.get("type") == PLAN_SCOPE_APPLICATIONS:
            named = app_name in (scope.get("applicationRefs") or [])
        elif scope.get("type") == PLAN_SCOPE_NAMESPACES:
            named = bool(set(namespaces) & set(scope.get("namespaces") or []))
        else:
            named = False
        if named and not _excludes_every_workload(plan, workloads):
            out.append(plan)
    return out


def _production_uncovered(ctx: _Ctx, target) -> Result:
    if not ctx.production or ctx.covering:
        return [(_app_subject(ctx), None)]
    # No plan covers the app, so no environment name counts: a namespace name matched the production pattern.
    refs = [_object_ref(K8S_KIND_NAMESPACE, ns) for ns in ctx.inputs.namespaces
            if ctx.settings.production_pattern.search(ns)]
    return _app_finding(ctx, "protection.production_uncovered", INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, refs,
                        namespaces=_join(ctx.inputs.namespaces), pattern=ctx.settings.production_pattern.pattern)


def _production_audit_only(ctx: _Ctx, target) -> Result:
    active = [p for p in ctx.covering if p.get("phase") == PLAN_PHASE_ACTIVE]
    if not ctx.production or not active or any(p.get("mode") != PLAN_MODE_AUDIT for p in active):
        return [(_app_subject(ctx), None)]
    refs = [EvidenceRef(type=EVIDENCE_TYPE_PLAN, ref=REF_PLAN.format(id=p.get("id"))) for p in active]
    return _app_finding(ctx, "protection.production_audit_only", INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH, refs,
                        plans=_join(p.get("name") or p.get("id") for p in active))


def _image_key(image: str) -> tuple:
    return image_parts(image) or (image,)


def _image_skew(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if ctx.inputs.capped:
        return []
    twins = sorted((w for w in ctx.inputs.workloads.values() if w.subject == wl.subject), key=lambda w: w.namespace)
    if len(twins) < 2:
        return _none(wl)
    # One card per subject, kept by the first namespace.
    if wl is not twins[0]:
        return []
    images = {w.namespace: [_image_key(c.get("image") or "") for c in _containers(w)] for w in twins}
    if len({tuple(v) for v in images.values()}) == 1:
        return _none(wl)
    refs = [EvidenceRef(type=EVIDENCE_TYPE_SPEC, ref=REF_SPEC.format(subject=wl.subject, path=f"containers@{w.namespace}"))
            for w in twins]
    shown = [c.get("image") for w in twins for c in _containers(w)]
    return _wl_finding("consistency.image_skew", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH, refs,
                       namespaces=_join(w.namespace for w in twins), images=_join(shown))


# ---- v1 rules, continued ------------------------------------------------------------------------------------
def _revision_history_zero(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if wl.kind != WORKLOAD_DEPLOYMENT or _spec(wl).get("revisionHistoryLimit") != 0:
        return _none(wl)
    return _wl_finding("reliability.revision_history_zero", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "revisionHistoryLimit")])


def _deployment_paused(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if wl.kind != WORKLOAD_DEPLOYMENT or _spec(wl).get("paused") is not True:
        return _none(wl)
    return _wl_finding("reliability.deployment_paused", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_spec_ref(wl, "paused")])


def _liveness_single_failure(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _containers(wl) if (c.get("livenessProbe") or {}).get("failureThreshold") == 1]
    if not names:
        return _none(wl)
    return _wl_finding("reliability.liveness_single_failure", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "livenessProbe"), containers=_join(names))


def _probe_port_undeclared(ctx: _Ctx, wl: WorkloadInput) -> Result:
    """A named probe port the container does not declare: the kubelet cannot resolve it, the check always fails.
    A numeric port never counts (declaring ports is optional)."""
    for c in _containers(wl):
        for probe_kind in PROBE_KINDS:
            probe = c.get(probe_kind) or {}
            port = next(((probe.get(h) or {}).get("port") for h in PROBE_PORT_HANDLERS if probe.get(h)), None)
            if isinstance(port, str) and not _port_matches(port, c):
                return _wl_finding("reliability.probe_port_undeclared", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                                   _container_refs(wl, [c.get("name")], probe_kind), containers=c.get("name"),
                                   probe=probe_kind.removesuffix(PROBE_SUFFIX), port=port)
    return _none(wl)


def _required_available(value, replicas: int) -> int | None:
    if isinstance(value, int):
        return value
    if isinstance(value, str) and value.endswith(PERCENT_SUFFIX) and value[:-1].isdigit():
        return math.ceil(replicas * int(value[:-1]) / FULL_PERCENT)
    return None


def _pdb_blocks_at_min_scale(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    if hpa is None:
        return _none(wl)
    low = _hpa_bounds(hpa)[0]
    for pdb in _matching_pdbs(ctx, wl):
        spec, status = pdb.get("spec") or {}, pdb.get("status") or {}
        required = _required_available(spec.get("minAvailable"), low)
        blocking_now = (status.get("disruptionsAllowed") == 0
                        and status.get("currentHealthy", 0) >= status.get("expectedPods", 1) > 0)
        if required is not None and required >= low and not blocking_now:
            return _wl_finding("reliability.pdb_blocks_at_min_scale", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                               [_object_ref(K8S_KIND_PDB, _name(pdb)), _object_ref(K8S_KIND_HPA, _name(hpa))],
                               pdb=_name(pdb), hpa=_name(hpa), minAvailable=spec.get("minAvailable"), replicas=low)
    return _none(wl)


def _hpa_inactive(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    if hpa is None:
        return _none(wl)
    inactive = next((c for c in (hpa.get("status") or {}).get("conditions") or []
                     if c.get("type") == CONDITION_SCALING_ACTIVE and c.get("status") == CONDITION_FALSE_STATUS
                     and c.get("reason") != REASON_SCALING_DISABLED), None)
    # Missing requests explain it already (scaling.hpa_missing_requests).
    if inactive is None or _hpa_missing_requests(ctx, wl)[0][1] is not None:
        return _none(wl)
    return _wl_finding("scaling.hpa_inactive", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       [_object_ref(K8S_KIND_HPA, _name(hpa))], hpa=_name(hpa), condition=inactive.get("reason"))


def _seccomp_unset(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl) if not _is_privileged(c)
             and (_sc(c, wl, "seccompProfile") or {}).get("type") in (None, SECCOMP_UNCONFINED)]
    if not names:
        return _none(wl)
    return _wl_finding("security.seccomp_unset", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext/seccompProfile"), containers=_join(names))


def _capabilities_not_dropped(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl) if not _is_privileged(c) and CAPABILITY_ALL not in
             [_capability(d) for d in ((c.get("securityContext") or {}).get("capabilities") or {}).get("drop") or []]]
    if not names:
        return _none(wl)
    return _wl_finding("security.capabilities_not_dropped", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext/capabilities"), containers=_join(names))


def _host_port(ctx: _Ctx, wl: WorkloadInput) -> Result:
    # With hostNetwork the API sets hostPort = containerPort on every port: security.host_namespaces says it.
    if _pod_spec(wl).get("hostNetwork") is True:
        return _none(wl)
    bound = {c.get("name"): [p["hostPort"] for p in _declared_ports(c) if p.get("hostPort")] for c in _containers(wl)}
    bound = {n: ports for n, ports in bound.items() if ports}
    if not bound:
        return _none(wl)
    return _wl_finding("security.host_port", wl, _node_agent(wl, INSIGHT_SEVERITY_WARNING), CONFIDENCE_HIGH,
                       _container_refs(wl, list(bound), "ports"), containers=_join(bound),
                       ports=_join(p for ports in bound.values() for p in ports))


def _run_as_root_group(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl) if _sc(c, wl, "runAsGroup") == 0]
    if not names:
        return _none(wl)
    return _wl_finding("security.run_as_root_group", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext"), containers=_join(names))


def _proc_mount_unmasked(ctx: _Ctx, wl: WorkloadInput) -> Result:
    names = [c.get("name") for c in _all_containers(wl)
             if (c.get("securityContext") or {}).get("procMount") == PROC_MOUNT_UNMASKED]
    if not names:
        return _none(wl)
    return _wl_finding("security.proc_mount_unmasked", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, names, "securityContext/procMount"), containers=_join(names))


def _pull_policy_never(ctx: _Ctx, wl: WorkloadInput) -> Result:
    # A moving tag with Never is images.pull_policy_mismatch.
    never = [c for c in _containers(wl) if c.get("imagePullPolicy") == PULL_POLICY_NEVER
             and not _moving(c.get("image") or "")]
    if not never:
        return _none(wl)
    return _wl_finding("images.pull_policy_never", wl, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH,
                       _container_refs(wl, [c.get("name") for c in never], "imagePullPolicy"),
                       containers=_join(c.get("name") for c in never), image=never[0].get("image"))


def _digest_not_pinned_production(ctx: _Ctx, wl: WorkloadInput) -> Result:
    if not _wl_production(ctx, wl):
        return _none(wl)
    # Moving tags are images.mutable_tag; only a version tag without a digest counts here.
    tagged = [c for c in _containers(wl) if (parts := image_parts(c.get("image") or "")) and not parts[3]
              and parts[2] not in ("", IMAGE_TAG_LATEST)]
    if not tagged:
        return _none(wl)
    return _wl_finding("images.digest_not_pinned_production", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_MEDIUM,
                       _container_refs(wl, [c.get("name") for c in tagged], "image"),
                       containers=_join(c.get("name") for c in tagged), image=tagged[0].get("image"))


def _subpath_no_reload(ctx: _Ctx, wl: WorkloadInput) -> Result:
    config = {v.get("name") for v in _pod_spec(wl).get("volumes") or [] if any(k in v for k in CONFIG_VOLUME_SOURCES)}
    hits = {c.get("name"): [m.get("name") for m in c.get("volumeMounts") or []
                            if m.get("name") in config and (m.get("subPath") or m.get("subPathExpr"))]
            for c in _all_containers(wl)}
    hits = {n: vols for n, vols in hits.items() if vols}
    if not hits:
        return _none(wl)
    return _wl_finding("config.subpath_no_reload", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       _container_refs(wl, list(hits), "volumeMounts"), containers=_join(hits),
                       volumes=_join(v for vols in hits.values() for v in vols))


def _hpa_scale_down_disabled(ctx: _Ctx, wl: WorkloadInput) -> Result:
    hpa = _hpa(ctx, wl)
    down = ((((hpa or {}).get("spec") or {}).get("behavior") or {}).get("scaleDown") or {})
    if hpa is None or down.get("selectPolicy") != SELECT_POLICY_DISABLED:
        return _none(wl)
    return _wl_finding("scaling.hpa_scale_down_disabled", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                       [_object_ref(K8S_KIND_HPA, _name(hpa))], hpa=_name(hpa))


def _network_policy_allows_all(ctx: _Ctx, wl: WorkloadInput) -> Result:
    for policy in ctx.inputs.items(FAMILY_NETPOLS, wl.namespace) or []:
        spec = policy.get("spec") or {}
        if not selector_matches(spec.get("podSelector", {}), _labels(wl)):
            continue
        if spec.get("policyTypes") and POLICY_TYPE_INGRESS not in spec["policyTypes"]:
            continue
        if any(not rule.get("from") and not rule.get("ports") for rule in spec.get("ingress") or []):
            return _wl_finding("networking.network_policy_allows_all", wl, INSIGHT_SEVERITY_INFO, CONFIDENCE_HIGH,
                               [_object_ref(K8S_KIND_NETWORK_POLICY, _name(policy))], policy=_name(policy))
    return _none(wl)


# ---- registry and evaluation ---------------------------------------------------------------------------------
def _rule(families: str, scope: str, fn: Callable) -> Rule:
    return Rule(frozenset(families), scope, fn)


W, S, P, H, N, U, A, D, X = (FAMILY_WORKLOADS, FAMILY_SERVICES, FAMILY_PDBS, FAMILY_HPAS, FAMILY_NETPOLS,
                             FAMILY_USAGE, FAMILY_APP, FAMILY_DOCUMENT, FAMILY_PLANS)
RULES: dict[str, Rule] = {
    "reliability.single_replica": _rule(W + H, SCOPE_WORKLOAD, _single_replica),
    "reliability.no_pdb": _rule(W + P + H, SCOPE_WORKLOAD, _no_pdb),
    "reliability.pdb_blocks_eviction": _rule(W + P, SCOPE_WORKLOAD, _pdb_blocks_eviction),
    "reliability.no_readiness_probe": _rule(W + S, SCOPE_WORKLOAD, _no_readiness_probe),
    "reliability.no_liveness_probe": _rule(W, SCOPE_WORKLOAD, _no_liveness_probe),
    "reliability.liveness_same_as_readiness": _rule(W, SCOPE_WORKLOAD, _liveness_same_as_readiness),
    # E (this run's events) is needed to find; without it only the resolve branch runs (a startupProbe added).
    "reliability.no_startup_probe": _rule(W, SCOPE_WORKLOAD, _no_startup_probe),
    "reliability.replicas_same_node": _rule(W, SCOPE_WORKLOAD, _replicas_same_node),
    "reliability.rollout_all_at_once": _rule(W, SCOPE_WORKLOAD, _rollout_all_at_once),
    "reliability.short_grace_period": _rule(W, SCOPE_WORKLOAD, _short_grace_period),
    "resources.no_requests": _rule(W, SCOPE_WORKLOAD, _no_requests),
    "resources.no_memory_limit": _rule(W, SCOPE_WORKLOAD, _no_memory_limit),
    "resources.limits_without_requests": _rule(W, SCOPE_WORKLOAD, _limits_without_requests),
    RECOMMENDATION_MEMORY_NEAR_LIMIT: _rule(W + U, SCOPE_WORKLOAD, _near_limit(RECOMMENDATION_MEMORY_NEAR_LIMIT,
                                                                               RESOURCE_MEMORY, CONFIDENCE_HIGH)),
    RECOMMENDATION_CPU_NEAR_LIMIT: _rule(W + U, SCOPE_WORKLOAD, _near_limit(RECOMMENDATION_CPU_NEAR_LIMIT,
                                                                            RESOURCE_CPU, CONFIDENCE_MEDIUM)),
    RECOMMENDATION_OVERPROVISIONED: _rule(W + U, SCOPE_WORKLOAD, _overprovisioned),
    RECOMMENDATION_UNDERPROVISIONED: _rule(W + U, SCOPE_WORKLOAD, _underprovisioned),
    "resources.oom_history": _rule(W + D, SCOPE_WORKLOAD, _oom_history),
    "scaling.hpa_min_equals_max": _rule(H, SCOPE_WORKLOAD, _hpa_min_equals_max),
    "scaling.hpa_missing_requests": _rule(W + H, SCOPE_WORKLOAD, _hpa_missing_requests),
    "scaling.hpa_at_max": _rule(H, SCOPE_WORKLOAD, _hpa_at_max),
    "scaling.no_hpa_sustained_load": _rule(W + H + U, SCOPE_WORKLOAD, _no_hpa_sustained_load),
    "security.privileged": _rule(W, SCOPE_WORKLOAD, _privileged),
    "security.privilege_escalation_allowed": _rule(W, SCOPE_WORKLOAD, _privilege_escalation),
    "security.runs_as_root": _rule(W, SCOPE_WORKLOAD, _runs_as_root),
    "security.writable_root_fs": _rule(W, SCOPE_WORKLOAD, _writable_root_fs),
    "security.added_capabilities": _rule(W, SCOPE_WORKLOAD, _added_capabilities),
    "security.host_namespaces": _rule(W, SCOPE_WORKLOAD, _host_namespaces),
    "security.host_path": _rule(W, SCOPE_WORKLOAD, _host_path),
    "security.default_service_account": _rule(W, SCOPE_WORKLOAD, _default_service_account),
    "security.token_automount": _rule(W, SCOPE_WORKLOAD, _token_automount),
    "security.secrets_in_env": _rule(W, SCOPE_WORKLOAD, _secrets_in_env),
    "security.plaintext_secret_env": _rule(W, SCOPE_WORKLOAD, _plaintext_secret_env),
    "images.mutable_tag": _rule(W, SCOPE_WORKLOAD, _mutable_tag),
    "images.pull_policy_mismatch": _rule(W, SCOPE_WORKLOAD, _pull_policy_mismatch),
    "config.duplicate_env": _rule(W, SCOPE_WORKLOAD, _duplicate_env),
    "networking.service_selector_mismatch": _rule(W + S + A, SCOPE_SERVICE, _service_selector_mismatch),
    "networking.service_port_mismatch": _rule(W + S, SCOPE_SERVICE, _service_port_mismatch),
    "networking.no_network_policy": _rule(W + N, SCOPE_WORKLOAD, _no_network_policy),
    "change_risk.high_velocity": _rule(A, SCOPE_APP, _high_velocity),
    "change_risk.frequent_rollbacks": _rule(A, SCOPE_APP, _frequent_rollbacks),
    "protection.production_uncovered": _rule(A + X, SCOPE_APP, _production_uncovered),
    "protection.production_audit_only": _rule(A + X, SCOPE_APP, _production_audit_only),
    "consistency.image_skew": _rule(W, SCOPE_WORKLOAD, _image_skew),
    "reliability.revision_history_zero": _rule(W, SCOPE_WORKLOAD, _revision_history_zero),
    "reliability.deployment_paused": _rule(W, SCOPE_WORKLOAD, _deployment_paused),
    "reliability.liveness_single_failure": _rule(W, SCOPE_WORKLOAD, _liveness_single_failure),
    "reliability.probe_port_undeclared": _rule(W, SCOPE_WORKLOAD, _probe_port_undeclared),
    "reliability.pdb_blocks_at_min_scale": _rule(W + P + H, SCOPE_WORKLOAD, _pdb_blocks_at_min_scale),
    "scaling.hpa_inactive": _rule(W + H, SCOPE_WORKLOAD, _hpa_inactive),
    "security.seccomp_unset": _rule(W, SCOPE_WORKLOAD, _seccomp_unset),
    "security.capabilities_not_dropped": _rule(W, SCOPE_WORKLOAD, _capabilities_not_dropped),
    "security.host_port": _rule(W, SCOPE_WORKLOAD, _host_port),
    "security.run_as_root_group": _rule(W, SCOPE_WORKLOAD, _run_as_root_group),
    "security.proc_mount_unmasked": _rule(W, SCOPE_WORKLOAD, _proc_mount_unmasked),
    "images.pull_policy_never": _rule(W, SCOPE_WORKLOAD, _pull_policy_never),
    # Production comes from the namespaces, and from the covering plans' environments when X is complete.
    "images.digest_not_pinned_production": _rule(W, SCOPE_WORKLOAD, _digest_not_pinned_production),
    "config.subpath_no_reload": _rule(W, SCOPE_WORKLOAD, _subpath_no_reload),
    "scaling.hpa_scale_down_disabled": _rule(H, SCOPE_WORKLOAD, _hpa_scale_down_disabled),
    "networking.network_policy_allows_all": _rule(W + N, SCOPE_WORKLOAD, _network_policy_allows_all),
}
USAGE_RULES = (RECOMMENDATION_MEMORY_NEAR_LIMIT, RECOMMENDATION_CPU_NEAR_LIMIT, RECOMMENDATION_OVERPROVISIONED,
               RECOMMENDATION_UNDERPROVISIONED)
_RULE_ORDER = {reason: i for i, reason in enumerate(RULES)}


def _app_workloads(inputs: ReviewInputs) -> list[tuple[str, str, str]]:
    """(namespace, kind, name) of every workload the app groups outside the excluded namespaces, read or not,
    except those a GET found gone."""
    listed = [(r.get("namespace"), (r.get("kind") or "").lower(), r.get("name"))
              for r in inputs.app.get("resources") or []
              if (r.get("kind") or "").lower() in WORKLOAD_KINDS and r.get("namespace") not in inputs.excluded]
    return [w for w in listed if w not in inputs.gone]


def _context(inputs: ReviewInputs, now: datetime, settings: Settings) -> _Ctx:
    items = (inputs.app.get("namespaces") or {}).get("items") or []
    ctx = _Ctx(inputs, now, settings, app_name=inputs.app.get("name") or "",
               primary=items[0].get("name") or "" if items else "")
    env_names: list[str] = []
    if FAMILY_PLANS in inputs.complete:
        ctx.covering = covering_plans(inputs.plans or [], ctx.app_name, inputs.namespaces, _app_workloads(inputs))
        env_names = [(inputs.env_names or {}).get(p.get("environmentRef"), "") for p in ctx.covering]
    ctx.production = is_production(inputs.namespaces, env_names, settings.production_pattern)
    return ctx


def card_key(reason: str, subject: str, params: dict[str, str]) -> str:
    """The identity a card was created under (the usage rules add container and resource)."""
    if reason in USAGE_RULES:
        return KEY_TEMPLATE.format(subject=subject, container=params.get("container"), resource=params.get("resource"))
    return subject


def _gone(ctx: _Ctx) -> set[tuple[str, str, str]]:
    """Keys of active cards whose subject no longer exists in a complete read: evaluated, and not found."""
    inputs = ctx.inputs
    present = set(_app_workloads(inputs))
    out = set()
    for card in inputs.doc.insights:
        rule = RULES.get(card.reason)
        if (card.category != INSIGHT_CATEGORY_RECOMMENDATION or card.status not in (INSIGHT_STATUS_OPEN, INSIGHT_STATUS_UPDATED)
                or rule is None or not rule.families <= inputs.complete):
            continue
        namespace = card.params.get("namespace", "")
        kind, _, name = card.subject.partition("/")
        if rule.scope == SCOPE_SERVICE:
            gone = namespace in inputs.excluded or (
                FAMILY_SERVICES in inputs.complete and namespace in inputs.namespaces
                and not any(_name(s) == name for s in inputs.items(FAMILY_SERVICES, namespace) or []))
        elif rule.scope == SCOPE_WORKLOAD:
            gone = (namespace, kind, name) not in present
            wl = inputs.workloads.get((namespace, card.subject))
            if not gone and wl is not None and card.reason in USAGE_RULES:
                gone = card.params.get("container") not in [c.get("name") for c in _containers(wl)]
        else:
            gone = False
        if gone:
            out.add((card.reason, namespace, card_key(card.reason, card.subject, card.params)))
    return out


def _rule_results(ctx: _Ctx, rule: Rule, targets: list) -> Iterator[tuple[str, str, Finding | None]]:
    """(namespace, key, finding or None) for every key the rule evaluates on the targets."""
    lists = rule.families & NAMESPACE_FAMILIES
    for target in targets:
        # A workload in a namespace beyond the listed ones: 'no PDB there' would only mean 'not read'.
        if rule.scope == SCOPE_WORKLOAD and any(ctx.inputs.items(f, target.namespace) is None for f in lists):
            continue
        namespace = (target.namespace if rule.scope == SCOPE_WORKLOAD
                     else target[0] if rule.scope == SCOPE_SERVICE else ctx.primary)
        for key, finding in rule.fn(ctx, target):
            yield namespace, key, finding


def evaluate(inputs: ReviewInputs, now: datetime, settings: Settings | None = None
             ) -> tuple[list[Finding], set[tuple[str, str, str]]]:
    """Findings, most severe first, and every evaluated key (reason, namespace, key)."""
    ctx = _context(inputs, now, settings or Settings())
    targets = {
        SCOPE_WORKLOAD: list(inputs.workloads.values()),
        SCOPE_SERVICE: _app_services(ctx),
        SCOPE_APP: [None],
    }
    findings: list[Finding] = []
    evaluated = _gone(ctx)
    for reason, rule in RULES.items():
        if not rule.families <= inputs.complete:
            continue
        for namespace, key, finding in _rule_results(ctx, rule, targets[rule.scope]):
            evaluated.add((reason, namespace, key))
            if finding is not None:
                findings.append(finding)
    findings.sort(key=lambda f: (-SEVERITY_RANK[f.severity], _RULE_ORDER[f.reason]))
    return findings, evaluated
