"""Tools answered from the in-cluster API: GET only, with the pod's service-account token.

The per-run caps bound traffic to 16 GETs per run (3 x 2 + 2 x 5), one run at a
time, so there is no client-side rate limiter.
"""

from __future__ import annotations

import re
from datetime import UTC, datetime, timedelta
from pathlib import Path

import httpx

from constants import (
    BEARER_TEMPLATE,
    CONDITION_DISRUPTION_TARGET,
    CONDITION_READY,
    CONDITION_TRUE,
    DAEMONSET_POD_NAME_PATTERN,
    DEPLOYMENT_POD_NAME_PATTERN,
    EVENT_DEDUPE_CHARS,
    EVENT_MESSAGE_MAX,
    EVENT_NAMESPACES_MAX,
    EVENTS_KEEP_MAX,
    EVENTS_LIST_LIMIT,
    HEADER_AUTHORIZATION,
    K8S_EVENTS_PATH,
    K8S_KIND_POD,
    K8S_KIND_REPLICASET,
    K8S_PARAM_FIELD_SELECTOR,
    K8S_PARAM_LABEL_SELECTOR,
    K8S_PARAM_LIMIT,
    K8S_PODS_PATH,
    K8S_WORKLOAD_PATH,
    POD_PHASE_PENDING,
    PODS_MAX,
    REF_EVENT,
    REF_WORKLOAD,
    REPLICASET_NAME_PATTERN,
    RECOVERED_MIN_UPTIME_S,
    RESOURCE_MEMORY,
    RFC3339_FORMAT,
    SA_TOKEN_PATH,
    STATEFULSET_POD_NAME_PATTERN,
    SUBJECT_SEPARATOR,
    SUBJECT_TEMPLATE,
    WARNING_FIELD_SELECTOR,
    WORKLOAD_DAEMONSET,
    WORKLOAD_DEPLOYMENT,
    WORKLOAD_KINDS,
)
from models import EventsArgs, Run, WorkloadArgs
from tools.app_tools import app_namespaces

KEY_PODS = "pods"
KEY_EVENTS = "events"
KEY_LIMITS = "limits"
KEY_PENDING = "pending"
KEY_FULL_IMAGES = "fullImages"
KEY_WAITING_MESSAGE = "waitingMessage"
KEY_DISRUPTION = "disruption"
# The untruncated event text, kept on run.events_cache entries only (never in a tool result, never logged).
KEY_FULL_MESSAGE = "fullMessage"
# The event's namespace, on run.events_cache entries only: it ties the event to one workload of the same name.
KEY_NAMESPACE = "namespace"
_PRIVATE_POD_KEYS = (KEY_WAITING_MESSAGE, KEY_DISRUPTION)
_PRIVATE_EVENT_KEYS = (KEY_FULL_MESSAGE, KEY_NAMESPACE)


class K8sError(Exception):
    """A non-2xx answer; carries the HTTP status only, never the body."""

    def __init__(self, status: int) -> None:
        super().__init__(status)
        self.status = status


class K8s:
    """The analyzer's only in-cluster API client; `get` is its only request method."""

    def __init__(self, client: httpx.AsyncClient) -> None:
        self._client = client

    async def get(self, path: str, params: dict | None = None) -> dict:
        # Projected tokens rotate: read the file on every call.
        token = Path(SA_TOKEN_PATH).read_text().strip()
        resp = await self._client.get(path, params=params, headers={HEADER_AUTHORIZATION: BEARER_TEMPLATE.format(token)})
        if not resp.is_success:
            raise K8sError(resp.status_code)
        return resp.json() or {}


def _pod_summary(pod: dict) -> dict:
    """The pod's state as the reported container shows it: the first container (init first) that waits, else
    the first with a last termination, so the exit code and the container name always agree."""
    status = pod.get("status") or {}
    inits = status.get("initContainerStatuses") or []
    containers = inits + (status.get("containerStatuses") or [])
    waiting = next((c for c in containers if ((c.get("state") or {}).get("waiting") or {}).get("reason")), None)
    reported = waiting or next((c for c in containers if (c.get("lastState") or {}).get("terminated")), None) or {}
    wait = (reported.get("state") or {}).get("waiting") or {}
    last = (reported.get("lastState") or {}).get("terminated")
    disruption = next((c.get("reason") for c in status.get("conditions") or []
                       if c.get("type") == CONDITION_DISRUPTION_TARGET), None)
    return {
        "name": (pod.get("metadata") or {}).get("name"),
        "phase": status.get("phase"),
        "restarts": sum(c.get("restartCount", 0) for c in containers),
        "waitingReason": wait.get("reason"),
        "lastTerminated": last and {"reason": last.get("reason"), "exitCode": last.get("exitCode"),
                                    "at": last.get("finishedAt")},
        "container": reported.get("name"),
        "init": any(reported is c for c in inits),
        "node": (pod.get("spec") or {}).get("nodeName"),
        # In memory only (status_cache): the model never sees them.
        KEY_WAITING_MESSAGE: wait.get("message"),
        KEY_DISRUPTION: disruption,
    }


def _public(pod: dict) -> dict:
    return {k: v for k, v in pod.items() if k not in _PRIVATE_POD_KEYS}


async def get_workload_status(run: Run, args: WorkloadArgs, k8s: K8s) -> tuple[dict, str]:
    key = run.bind(SUBJECT_TEMPLATE.format(kind=args.kind, name=args.name), args.namespace)
    kind, name, namespace = run.workloads[key]
    resource = WORKLOAD_KINDS[kind][1]
    try:
        obj = await k8s.get(K8S_WORKLOAD_PATH.format(namespace=namespace, resource=resource, name=name))
    except K8sError as e:
        if e.status == httpx.codes.NOT_FOUND:
            run.gone.add(key)
        raise
    spec, status = obj.get("spec") or {}, obj.get("status") or {}
    if kind == WORKLOAD_DAEMONSET:
        counts = (status.get("desiredNumberScheduled", 0), status.get("updatedNumberScheduled", 0),
                  status.get("numberReady", 0))
    else:
        counts = (spec.get("replicas", 0), status.get("updatedReplicas", 0), status.get("readyReplicas", 0))

    labels = (spec.get("selector") or {}).get("matchLabels") or {}
    items: list[dict] = []
    # No matchLabels: an empty selector would list every pod of the namespace.
    if labels:
        selector = ",".join(f"{k}={v}" for k, v in labels.items())
        body = await k8s.get(K8S_PODS_PATH.format(namespace=namespace), {K8S_PARAM_LABEL_SELECTOR: selector})
        items = body.get("items") or []
    pods = sorted((_pod_summary(p) for p in items), key=lambda p: p["restarts"], reverse=True)
    containers = ((spec.get("template") or {}).get("spec") or {}).get("containers") or []
    limits = {c.get("name"): mem for c in containers
              if (mem := ((c.get("resources") or {}).get("limits") or {}).get(RESOURCE_MEMORY))}

    payload = {
        "kind": kind,
        "name": name,
        "conditions": [{k: c.get(k) for k in ("type", "status", "reason")} for c in status.get("conditions") or []],
        "desired": counts[0],
        "updated": counts[1],
        "ready": counts[2],
        KEY_PODS: [_public(p) for p in pods[:PODS_MAX]],
        "images": [(c.get("image") or "").rsplit("/", 1)[-1] for c in containers],
        KEY_LIMITS: limits,
    }
    # The rules and the resolve read the full result, not the size-cut one the model sees; the review reuses the reads.
    run.status_cache[key] = {
        **payload, KEY_PODS: pods[:PODS_MAX],
        KEY_PENDING: sum(p["phase"] == POD_PHASE_PENDING for p in pods),
        KEY_FULL_IMAGES: {c.get("name"): c.get("image") or "" for c in containers},
    }
    run.spec_cache[key] = obj
    run.pods_cache[key] = items if labels else None
    return payload, KEY_PODS


def status_refs(payload: dict) -> list[str]:
    return [REF_WORKLOAD.format(kind=payload["kind"], name=payload["name"])]


def workload_matchers(kind: str, name: str) -> list[tuple[str, re.Pattern]]:
    """(involvedObject kind, exact name shape) for one workload and the objects it owns."""
    escaped = re.escape(name)
    out = [(WORKLOAD_KINDS[kind][0], re.compile(f"^{escaped}$"))]
    if kind == WORKLOAD_DEPLOYMENT:
        out.append((K8S_KIND_REPLICASET, re.compile(REPLICASET_NAME_PATTERN.format(name=escaped))))
        out.append((K8S_KIND_POD, re.compile(DEPLOYMENT_POD_NAME_PATTERN.format(name=escaped))))
    elif kind == WORKLOAD_DAEMONSET:
        out.append((K8S_KIND_POD, re.compile(DAEMONSET_POD_NAME_PATTERN.format(name=escaped))))
    else:
        out.append((K8S_KIND_POD, re.compile(STATEFULSET_POD_NAME_PATTERN.format(name=escaped))))
    return out


def stale_pod_event(status: dict | None, items: list[dict] | None, event: dict, now: datetime) -> bool:
    """For a fully ready workload, a pod event that is history rather than a symptom: its pod no longer exists (a
    replaced pod's leftover, for example the BackOff of the pod a fix rolled away), or its pod became Ready after
    the event last occurred (a start-up blip) and no restarted container of it is younger than
    RECOVERED_MIN_UPTIME_S (a crash loop's container is Ready for the seconds it runs between two back-offs)."""
    if status is None or items is None or status["ready"] < status["desired"]:
        return False
    kind, _, name = event["object"].partition(SUBJECT_SEPARATOR)
    if kind != K8S_KIND_POD:
        return False
    pod = next((p for p in items if (p.get("metadata") or {}).get("name") == name), None)
    if pod is None:
        return True
    pod_status = pod.get("status") or {}
    ready = next((c for c in pod_status.get("conditions") or [] if c.get("type") == CONDITION_READY), {})
    # Both sides are RFC3339 UTC timestamps of the same width: string order is time order.
    if not (ready.get("status") == CONDITION_TRUE and event["last"] < (ready.get("lastTransitionTime") or "")):
        return False
    started = [_parse(((c.get("state") or {}).get("running") or {}).get("startedAt"))
               for c in pod_status.get("containerStatuses") or [] if c.get("restartCount")]
    return not any(s and (now - s).total_seconds() < RECOVERED_MIN_UPTIME_S for s in started)


def _matchers(run: Run, namespace: str) -> list[tuple[str, re.Pattern]]:
    """Matchers of every app workload in the namespace."""
    return [m for kind, name, ns in run.workloads.values() if ns == namespace for m in workload_matchers(kind, name)]


def _parse(value: str | None) -> datetime | None:
    return datetime.fromisoformat(value) if value else None


def _last_seen(event: dict) -> datetime | None:
    seen = [
        _parse(event.get("lastTimestamp")),
        _parse(event.get("eventTime")),
        _parse((event.get("series") or {}).get("lastObservedTime")),
    ]
    return max((s for s in seen if s), default=None)


async def get_recent_events(run: Run, args: EventsArgs, k8s: K8s) -> tuple[dict, str]:
    namespaces = [args.namespace] if args.namespace else app_namespaces(run)[:EVENT_NAMESPACES_MAX]
    since = datetime.now(UTC) - timedelta(minutes=args.sinceMinutes)
    params = {K8S_PARAM_LIMIT: EVENTS_LIST_LIMIT}
    if args.warningsOnly:
        params[K8S_PARAM_FIELD_SELECTOR] = WARNING_FIELD_SELECTOR

    merged: dict[tuple, dict] = {}
    for namespace in namespaces:
        matchers = _matchers(run, namespace)
        body = await k8s.get(K8S_EVENTS_PATH.format(namespace=namespace), params)
        for event in body.get("items") or []:
            obj = event.get("involvedObject") or {}
            kind, name = obj.get("kind"), obj.get("name") or ""
            if not any(kind == k and pattern.match(name) for k, pattern in matchers):
                continue
            last = _last_seen(event)
            if last is None or last < since:
                continue
            first = _parse(event.get("firstTimestamp") or event.get("eventTime")) or last
            count = event.get("count") or (event.get("series") or {}).get("count") or 1
            message = event.get("message") or ""
            key = (event.get("reason"), SUBJECT_TEMPLATE.format(kind=kind, name=name), message[:EVENT_DEDUPE_CHARS],
                   namespace)
            if key in merged:
                seen = merged[key]
                seen["count"] += count
                seen["first"] = min(seen["first"], first)
                seen["last"] = max(seen["last"], last)
                continue
            merged[key] = {"reason": key[0], "object": key[1], "message": message[:EVENT_MESSAGE_MAX],
                           "count": count, "first": first, "last": last, KEY_FULL_MESSAGE: message,
                           KEY_NAMESPACE: namespace}

    events = sorted(merged.values(), key=lambda e: e["last"], reverse=True)[:EVENTS_KEEP_MAX]
    for event in events:
        event["first"] = event["first"].astimezone(UTC).strftime(RFC3339_FORMAT)
        event["last"] = event["last"].astimezone(UTC).strftime(RFC3339_FORMAT)
    run.events_cache.extend(events)
    return {KEY_EVENTS: [{k: v for k, v in e.items() if k not in _PRIVATE_EVENT_KEYS} for e in events]}, KEY_EVENTS


def events_refs(payload: dict) -> list[str]:
    return [REF_EVENT.format(**e) for e in payload[KEY_EVENTS]]
