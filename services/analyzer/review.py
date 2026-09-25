"""The review's read-only gather: everything the recommendation rules read, with a completeness flag per family.

GET only, like the tools: the app's workloads and their pods (reused from this run's
status reads when present), four lists per app namespace (Services, PodDisruptionBudgets,
HorizontalPodAutoscalers, NetworkPolicies), one pod probe per selector Service, the
protection plans from the exporter and the analyzer's own usage samples. A read that
fails, times out, is truncated or comes after the review wall leaves its family
incomplete: its rules then neither create nor resolve a card. A workload GET that
answers 404 is no failure: the workload is gone, and its cards resolve.
"""

from __future__ import annotations

import asyncio
import json
import time
from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import datetime

import httpx
from redis import RedisError

import exporter
from constants import (
    EVENT_NAMESPACES_MAX,
    FAMILY_APP,
    FAMILY_DOCUMENT,
    FAMILY_EVENTS,
    FAMILY_PLANS,
    FAMILY_SERVICES,
    FAMILY_USAGE,
    FAMILY_WORKLOADS,
    INSIGHT_CATEGORY_RECOMMENDATION,
    INSIGHT_STATUS_RESOLVED,
    JSON_COMPACT_SEPARATORS,
    K8S_PARAM_LABEL_SELECTOR,
    K8S_PARAM_LIMIT,
    K8S_PODS_PATH,
    K8S_WORKLOAD_PATH,
    KIND_SERVICE,
    LIST_CONTINUE_FIELD,
    NAMESPACE_LIST_LIMIT,
    NAMESPACE_LISTS,
    REVIEW_WALL_S,
    SELECTOR_EXPRESSION_TEMPLATES,
    SELECTOR_SEPARATOR,
    SUBJECT_TEMPLATE,
    TOOL_GET_RECENT_EVENTS,
    TOOL_TIMEOUT_S,
    USAGE_CPU_MILLI,
    USAGE_FIELD_TEMPLATE,
    USAGE_KEY,
    USAGE_MEM_BYTES,
    USAGE_SAMPLE_CONTAINERS,
    USAGE_SAMPLE_TIME,
    USAGE_SAMPLES_MAX,
    WORKLOAD_KINDS,
)
from config import ANALYZER_REVIEW_WORKLOADS_MAX, ANALYZER_USAGE_MIN_SAMPLES, ANALYZER_USAGE_MIN_SPAN_SEC
from exporter import ExporterUnavailable
from helpers import parse_cpu_milli, parse_mem_bytes
from models import AppInsights, Run
from tools.app_tools import app_namespaces
from tools.k8s_tools import K8s, K8sError

# A workload GET's answer for a 404, compared by identity.
_GONE: dict = {}


@dataclass
class WorkloadInput:
    kind: str
    name: str
    namespace: str
    subject: str
    obj: dict
    pods: list[dict]


@dataclass
class ReviewInputs:
    app: dict
    doc: AppInsights
    namespaces: list[str]
    excluded: list[str] = field(default_factory=list)
    # (namespace, subject) -> the workload object and its pods
    workloads: dict[tuple[str, str], WorkloadInput] = field(default_factory=dict)
    # (namespace, kind, name) of listed workloads whose GET answered 404: the Application's list outlived them.
    gone: set[tuple[str, str, str]] = field(default_factory=set)
    # True when the app has more workloads than ANALYZER_REVIEW_WORKLOADS_MAX (rules over all of them skip)
    capped: bool = False
    # family -> namespace -> items (None = that list is incomplete)
    lists: dict[str, dict[str, list[dict] | None]] = field(default_factory=dict)
    # (namespace, service) -> pods its selector matches (0 or 1, limit=1); None = unread
    selector_pods: dict[tuple[str, str], int | None] = field(default_factory=dict)
    # (namespace, subject) -> usage samples, oldest first
    usage: dict[tuple[str, str], list[dict]] = field(default_factory=dict)
    plans: list[dict] | None = None
    env_names: dict[str, str] | None = None
    events: list[dict] = field(default_factory=list)
    complete: set[str] = field(default_factory=set)
    gets: int = 0

    def items(self, family: str, namespace: str) -> list[dict] | None:
        return self.lists.get(family, {}).get(namespace)


class _Reader:
    """K8s.get with the per-read timeout and the review wall; None for any failure, `not_found` for a 404.
    Counts every request."""

    def __init__(self, k8s: K8s, inputs: ReviewInputs, deadline: float, clock: Callable[[], float]) -> None:
        self._k8s, self._inputs, self._deadline, self._clock = k8s, inputs, deadline, clock

    async def get(self, path: str, params: dict | None = None, not_found: dict | None = None) -> dict | None:
        if self._clock() >= self._deadline:
            return None
        self._inputs.gets += 1
        try:
            async with asyncio.timeout(TOOL_TIMEOUT_S):
                return await self._k8s.get(path, params)
        except K8sError as e:
            return not_found if e.status == httpx.codes.NOT_FOUND else None
        except (TimeoutError, httpx.HTTPError, ValueError):
            return None

    async def list(self, path: str, params: dict | None = None) -> list[dict] | None:
        """A list read; truncated (a continue token) counts as failed."""
        body = await self.get(path, {K8S_PARAM_LIMIT: NAMESPACE_LIST_LIMIT, **(params or {})})
        if body is None or (body.get("metadata") or {}).get(LIST_CONTINUE_FIELD):
            return None
        return body.get("items") or []


def selector_string(selector: dict | None) -> str:
    """A metav1.LabelSelector as a labelSelector query; '' for an empty or missing one."""
    selector = selector or {}
    parts = [f"{k}={v}" for k, v in (selector.get("matchLabels") or {}).items()]
    for expr in selector.get("matchExpressions") or []:
        template = SELECTOR_EXPRESSION_TEMPLATES.get(expr.get("operator"))
        if template:
            parts.append(template.format(key=expr.get("key"), values=SELECTOR_SEPARATOR.join(expr.get("values") or [])))
    return SELECTOR_SEPARATOR.join(parts)


def _workload_entries(app: dict, doc: AppInsights, excluded: list[str]) -> list[tuple[str, str, str]]:
    """(namespace, kind, name) of the app's workloads: those with an active incident first, then overview order."""
    entries = list(dict.fromkeys(
        (res.get("namespace"), (res.get("kind") or "").lower(), res.get("name"))
        for res in app.get("resources") or []
        if (res.get("kind") or "").lower() in WORKLOAD_KINDS and res.get("namespace") not in excluded))
    incident = [(c.params.get("namespace"), c.subject) for c in doc.insights
                if c.category != INSIGHT_CATEGORY_RECOMMENDATION and c.status != INSIGHT_STATUS_RESOLVED]
    # A card that names no namespace predates incident ids with one: it matches the workload in any namespace.
    return sorted(entries, key=lambda e: not any(
        subject == SUBJECT_TEMPLATE.format(kind=e[1], name=e[2]) and ns in (None, e[0]) for ns, subject in incident))


async def _workloads(inputs: ReviewInputs, reader: _Reader, run: Run, entries: list[tuple[str, str, str]]) -> bool:
    ok = True
    for namespace, kind, name in entries:
        subject = SUBJECT_TEMPLATE.format(kind=kind, name=name)
        # This run's status read of the same workload (same namespace) is reused: no second GET.
        obj = _GONE if (namespace, subject) in run.gone else run.spec_cache.get((namespace, subject))
        if obj is None:
            obj = await reader.get(K8S_WORKLOAD_PATH.format(namespace=namespace, resource=WORKLOAD_KINDS[kind][1],
                                                            name=name), not_found=_GONE)
        if obj is _GONE:
            inputs.gone.add((namespace, kind, name))
            continue
        pods = run.pods_cache.get((namespace, subject))
        if obj is not None and pods is None:
            selector = selector_string((obj.get("spec") or {}).get("selector"))
            pods = await reader.list(K8S_PODS_PATH.format(namespace=namespace),
                                     {K8S_PARAM_LABEL_SELECTOR: selector}) if selector else []
        if obj is None or pods is None:
            ok = False
            continue
        inputs.workloads[(namespace, subject)] = WorkloadInput(kind, name, namespace, subject, obj, pods)
    return ok


async def _selector_pods(inputs: ReviewInputs, reader: _Reader, excluded: list[str]) -> None:
    for res in inputs.app.get("resources") or []:
        namespace, name = res.get("namespace"), res.get("name")
        if (res.get("kind") or "").lower() != KIND_SERVICE or namespace in excluded:
            continue
        service = next((s for s in inputs.items(FAMILY_SERVICES, namespace) or []
                        if (s.get("metadata") or {}).get("name") == name), None)
        selector = ((service or {}).get("spec") or {}).get("selector") or {}
        if not selector:
            continue
        query = SELECTOR_SEPARATOR.join(f"{k}={v}" for k, v in selector.items())
        body = await reader.get(K8S_PODS_PATH.format(namespace=namespace),
                                {K8S_PARAM_LABEL_SELECTOR: query, K8S_PARAM_LIMIT: 1})
        inputs.selector_pods[(namespace, name)] = None if body is None else len(body.get("items") or [])


def _epoch(value: str | None) -> float | None:
    try:
        return datetime.fromisoformat(value).timestamp() if value else None
    except ValueError:
        return None


def _sample(usage: dict) -> dict | None:
    """One sample of a workload: per container, the max over its instances; None when nothing parses."""
    t = _epoch(usage.get("timestamp"))
    if not usage.get("available") or t is None:
        return None
    containers: dict[str, dict] = {}
    for instance in (usage.get("resources") or {}).get("usagePerInstance") or []:
        for c in instance.get("containers") or []:
            seen = containers.setdefault(c.get("name"), {USAGE_CPU_MILLI: None, USAGE_MEM_BYTES: None})
            for key, value in ((USAGE_CPU_MILLI, parse_cpu_milli(c.get("cpu"))),
                               (USAGE_MEM_BYTES, parse_mem_bytes(c.get("memory")))):
                if value is not None:
                    seen[key] = max(value, seen[key] or 0)
    return {USAGE_SAMPLE_TIME: t, USAGE_SAMPLE_CONTAINERS: containers} if containers else None


def sufficient(samples: list[dict]) -> bool:
    """Enough usage history to judge: ANALYZER_USAGE_MIN_SAMPLES samples over ANALYZER_USAGE_MIN_SPAN_SEC."""
    return (len(samples) >= ANALYZER_USAGE_MIN_SAMPLES
            and samples[-1][USAGE_SAMPLE_TIME] - samples[0][USAGE_SAMPLE_TIME] >= ANALYZER_USAGE_MIN_SPAN_SEC)


async def record_usage(redis, app_key: str, app: dict) -> dict[tuple[str, str], list[dict]]:
    """Append this Application's usage to the app's samples (newer timestamps only, the newest USAGE_SAMPLES_MAX)."""
    raw = await redis.hget(USAGE_KEY, app_key)
    stored: dict[str, list[dict]] = json.loads(raw) if raw else {}
    changed = False
    for w in (app.get("metrics") or {}).get("workloads") or []:
        sample = _sample(w.get("usage") or {})
        if sample is None:
            continue
        key = USAGE_FIELD_TEMPLATE.format(namespace=w.get("namespace"), kind=(w.get("resourceKind") or "").lower(),
                                          name=w.get("resourceName"))
        samples = stored.get(key, [])
        if samples and samples[-1][USAGE_SAMPLE_TIME] >= sample[USAGE_SAMPLE_TIME]:
            continue
        stored[key] = (samples + [sample])[-USAGE_SAMPLES_MAX:]
        changed = True
    if changed:
        await redis.hset(USAGE_KEY, app_key, json.dumps(stored, separators=JSON_COMPACT_SEPARATORS))
    out = {}
    for key, samples in stored.items():
        namespace, _, subject = key.partition("/")
        out[(namespace, subject)] = samples
    return out


async def gather(app: dict, doc: AppInsights, run: Run, k8s: K8s | None, exporter_client: httpx.AsyncClient,
                 redis, ns_cache: dict | None = None, in_run: bool = False,
                 clock: Callable[[], float] = time.monotonic) -> ReviewInputs:
    """Every input of one app's review. `ns_cache` (one per sweep tick) shares the namespace lists between apps."""
    deadline = clock() + REVIEW_WALL_S
    inputs = ReviewInputs(app=app, doc=doc, namespaces=app_namespaces(run)[:EVENT_NAMESPACES_MAX],
                          excluded=run.excluded)
    inputs.complete |= {FAMILY_APP, FAMILY_DOCUMENT}
    # Only an analysis run read this app's events (fast path, last hour); a sweep review has none.
    if in_run and run.calls[TOOL_GET_RECENT_EVENTS] and not run.truncated:
        inputs.events = list(run.events_cache)
        inputs.complete.add(FAMILY_EVENTS)
    try:
        inputs.plans, inputs.env_names = await exporter.plans_snapshot(exporter_client, time.time())
        inputs.complete.add(FAMILY_PLANS)
    except ExporterUnavailable:
        pass
    try:
        inputs.usage = await record_usage(redis, f"{run.namespace}/{run.name}", app)
        inputs.complete.add(FAMILY_USAGE)
    except (RedisError, ValueError):
        pass
    if k8s is None:
        return inputs

    reader = _Reader(k8s, inputs, deadline, clock)
    cache = {} if ns_cache is None else ns_cache
    for family, path in NAMESPACE_LISTS.items():
        lists = inputs.lists.setdefault(family, {})
        for namespace in inputs.namespaces:
            if (family, namespace) not in cache:
                items = await reader.list(path.format(namespace=namespace))
                # Only a complete list is shared: a failure is this app's, the next one reads again.
                if items is None:
                    lists[namespace] = None
                    continue
                cache[(family, namespace)] = items
            lists[namespace] = cache[(family, namespace)]
        if all(items is not None for items in lists.values()):
            inputs.complete.add(family)

    entries = _workload_entries(app, doc, run.excluded)
    inputs.capped = len(entries) > ANALYZER_REVIEW_WORKLOADS_MAX
    if await _workloads(inputs, reader, run, entries[:ANALYZER_REVIEW_WORKLOADS_MAX]):
        inputs.complete.add(FAMILY_WORKLOADS)
    await _selector_pods(inputs, reader, run.excluded)
    return inputs
