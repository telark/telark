"""Incident sub-reasons and the server-rendered title/summary, and the recommendations' texts.

Pure: `classify` reads exactly what the fast path gathered (the status payload with its
in-memory extras, the owned warning events with their untruncated text) and names the
reason plus the params every text is rendered from; `render_incident` builds the title
and summary from constants only. The UI renders cause and steps from the same reason
and params, so params carry every value a text needs, and nothing else.
"""

from __future__ import annotations

import re
import string
from collections import Counter

from constants import (
    ADMISSION_MARKERS,
    CHANGE_FIELD_REASONS,
    CHANGE_FIELD_RESOURCE_PREFIXES,
    CHANGE_REASON_OTHER,
    CHANGE_REASON_RESOURCES,
    COMBINED_EVENTS_PREFIX,
    CONDITION_FALSE,
    CONDITION_PROGRESSING,
    CRASHLOOP_MARKER,
    CREATE_CONTAINER_ERROR_REASONS,
    DEFAULT_REGISTRY,
    EVICTION_MARKER,
    EVICTION_MARKERS,
    EVICTION_OTHER,
    EXIT_CODE_REASONS,
    FAILED_CREATE_HEAD,
    IMAGE_IN_MESSAGE_PATTERN,
    IMAGE_PULL_MARKERS,
    IMAGE_PULL_MARKERS_BY_REASON,
    IMAGE_PULL_WAITING_REASONS,
    IMPACT_DOWN,
    IMPACT_PARTIAL,
    INCIDENT_DETAIL_FALLBACKS,
    INCIDENT_DETAIL_VARIANTS,
    INCIDENT_PARAM_KEYS,
    INCIDENT_TEXT,
    INSIGHT_KIND_CONFIG_CHANGE_REGRESSION,
    INSIGHT_KIND_CRASHLOOP,
    INSIGHT_KIND_IMAGE_PULL,
    INSIGHT_KIND_OOM,
    INSIGHT_KIND_OTHER,
    INSIGHT_KIND_PROBE_FAILURE,
    INSIGHT_KIND_RESOURCE_PRESSURE,
    INSIGHT_KIND_ROLLOUT_STUCK,
    INSIGHT_KIND_SCHEDULING,
    MAX_INSIGHT_PARAMS,
    MAX_INSIGHT_PARAM_LENGTH,
    MAX_INSIGHT_SUMMARY_LENGTH,
    MAX_INSIGHT_TITLE_LENGTH,
    OOM_EVENT_REASONS,
    PARAM_ELLIPSIS,
    PROBE_FAILURE_COMMAND,
    PROBE_FAILURE_MARKERS,
    PROBE_FAILURE_OTHER,
    PROBE_FAILURE_TEXT,
    PROBE_MESSAGE_PATTERN,
    PROBE_NAMES,
    PROBE_PORT_PATTERN,
    PROBE_READINESS,
    PROBE_STATUS_PATTERN,
    PROBE_TIMEOUT_PATTERN,
    PULL_CAUSE_TEMPLATE,
    QUOTA_MARKERS,
    REASON_BACKOFF,
    REASON_CRASHLOOP,
    REASON_CREATE_CONTAINER_CONFIG_ERROR,
    REASON_EVICTED,
    REASON_FAILED,
    REASON_FAILED_CREATE,
    REASON_FAILED_SCHEDULING,
    REASON_INVALID_IMAGE_NAME,
    REASON_OOM_KILLED,
    REASON_PREEMPTED,
    REASON_PREEMPTION_BY_SCHEDULER,
    REASON_PROGRESS_DEADLINE,
    REASON_SEPARATOR,
    REASON_START_ERROR,
    REASON_UNHEALTHY,
    RECOMMENDATION_TEXT,
    RESTART_PLURALS,
    RESTARTING_PROBES,
    RESTARTS_SINGULAR,
    ROOT_MODE_TEXT,
    RPC_ERROR_PREFIX_PATTERN,
    SCHEDULING_AVAILABLE_MARKER,
    SCHEDULING_COUNT_PATTERN,
    SCHEDULING_DRA_NOISE,
    SCHEDULING_OTHER,
    SCHEDULING_PIECE_SPLIT,
    SCHEDULING_PREEMPTION_MARKER,
    SCHEDULING_SEGMENTS,
    STATE_DEGRADED,
    STATE_DOWN,
    SUBJECT_SEPARATOR,
    TITLE_NO_STATE,
    TITLE_WITH_STATE,
    VOLUME_EVENT_REASONS,
    WARNING_REASONS_MAX,
    WARNING_REASONS_SEPARATOR,
)
from helpers import image_parts, is_registry_host
from tools.k8s_tools import KEY_DISRUPTION, KEY_FULL_IMAGES, KEY_FULL_MESSAGE, KEY_PENDING, KEY_WAITING_MESSAGE

Params = dict[str, object]


# ---- small readers ---------------------------------------------------------------------------------------------
def _text(event: dict) -> str:
    """The untruncated event text when the fast path kept it, else the 160-char cut."""
    return (event.get(KEY_FULL_MESSAGE) or event.get("message") or "").removeprefix(COMBINED_EVENTS_PREFIX)


def _head(message: str | None) -> str:
    return (message or "").strip().rstrip(".")


def _pod_of(event: dict) -> str:
    return event["object"].partition(SUBJECT_SEPARATOR)[2]


def _matches(patterns: tuple[str, ...], text: str) -> bool:
    return any(re.search(p, text) for p in patterns)


def _first(items, test):
    return next((i for i in items if test(i)), None)


def _last(pod: dict | None) -> dict:
    return (pod or {}).get("lastTerminated") or {}


def _replicas(status: dict | None) -> Params:
    return {"ready": status["ready"], "desired": status["desired"]} if status else {}


def _pod_params(pod: dict) -> Params:
    last = _last(pod)
    return {"pod": pod.get("name"), "container": pod.get("container"), "restarts": pod.get("restarts"),
            "exitCode": last.get("exitCode"), "lastReason": last.get("reason")}


def _event_params(events: list[dict]) -> Params:
    """What the newest event alone says: its pod, its text, how often."""
    if not events:
        return {}
    return {"pod": _pod_of(events[0]), "message": _head(_text(events[0])), "count": sum(e["count"] for e in events)}


def registry_of(image: str) -> str:
    """The registry host of a reference, also for an invalid one (the reason it cannot be pulled)."""
    if parts := image_parts(image):
        return parts[0]
    first, sep, _rest = image.partition("/")
    return first if sep and is_registry_host(first) else DEFAULT_REGISTRY


def probe_of(event: dict) -> str | None:
    text = _text(event).lower()
    if m := re.match(PROBE_MESSAGE_PATTERN, text):
        return m.group(1)
    return _first(PROBE_NAMES, lambda p: p in text)


def _probe_failure(text: str) -> Params:
    """failure + its value (port / status / timeout); the exec output of a 'command' failure is never kept."""
    lowered = text.lower()
    failure = _first([f for f, markers in PROBE_FAILURE_MARKERS if _matches(markers, lowered)], bool)
    failure = failure or PROBE_FAILURE_OTHER
    params: Params = {"failure": failure}
    extract = {"refused": ("port", PROBE_PORT_PATTERN), "http_status": ("status", PROBE_STATUS_PATTERN),
               "timeout": ("timeout", PROBE_TIMEOUT_PATTERN)}.get(failure)
    if extract and (m := re.search(extract[1], lowered)):
        params[extract[0]] = m.group(1)
    if failure != PROBE_FAILURE_COMMAND:
        params["message"] = _head(text)
    return params


def _pull_cause(message: str | None, image: str | None) -> str:
    """The registry's own words: what follows the last quoted reference of the pulled image."""
    message = message or ""
    if image and (at := message.rfind(PULL_CAUSE_TEMPLATE.format(image))) >= 0:
        message = message[at + len(PULL_CAUSE_TEMPLATE.format(image)):]
    return _head(re.sub(RPC_ERROR_PREFIX_PATTERN, "", message))


# ---- per kind --------------------------------------------------------------------------------------------------
def _image_pull(status, pods, events, limits, change) -> tuple[str, Params]:
    pod = _first(pods, lambda p: p.get("waitingReason") in IMAGE_PULL_WAITING_REASONS)
    evs = [e for e in events if e["reason"] in (REASON_FAILED, REASON_BACKOFF)
           and any(m in _text(e).lower() for m in IMAGE_PULL_MARKERS)]
    texts = [_text(e) for e in evs if e["reason"] == REASON_FAILED]
    texts += [pod.get(KEY_WAITING_MESSAGE) or ""] if pod else []
    texts += [_text(e) for e in evs if e["reason"] == REASON_BACKOFF]
    texts = [t for t in texts if t]
    image = _first([m.group(1) for t in texts if (m := re.search(IMAGE_IN_MESSAGE_PATTERN, t))], bool)
    if not image and pod and status:
        image = (status.get(KEY_FULL_IMAGES) or {}).get(pod.get("container"))
    # The image name itself may contain a marker ('app:not-found'): only the registry's words count.
    lowered = [t.lower().replace(image.lower(), " ") if image else t.lower() for t in texts]
    reason, message = "image_pull.other", _first(texts, bool)
    if pod and pod.get("waitingReason") == REASON_INVALID_IMAGE_NAME:
        reason = "image_pull.invalid_name"
    else:
        for candidate, markers in IMAGE_PULL_MARKERS_BY_REASON:
            hit = _first(range(len(texts)), lambda i: _matches(markers, lowered[i]))
            if hit is not None:
                reason, message = candidate, texts[hit]
                break
    params: Params = {**_replicas(status), "pod": pod["name"] if pod else _pod_of(evs[0]) if evs else None,
                      "container": pod and pod.get("container"), "message": _pull_cause(message, image)}
    if image:
        params.update(image=image, registry=registry_of(image))
    return reason, params


def _crashloop(status, pods, events, limits, change) -> tuple[str, Params]:
    probe_evs = [e for e in events if e["reason"] == REASON_UNHEALTHY and probe_of(e) in RESTARTING_PROBES]
    restarted = [p for p in pods if (p.get("restarts") or 0) > 0]
    if probe_evs and restarted:
        probe = probe_of(probe_evs[0])
        same = [e for e in probe_evs if probe_of(e) == probe]
        pod = _first(restarted, lambda p: p.get("name") == _pod_of(same[0])) or restarted[0]
        return "crashloop.probe_kill", {
            **_replicas(status), "pod": pod.get("name"), "container": pod.get("container"),
            "restarts": pod.get("restarts"), "probe": probe, "count": sum(e["count"] for e in same),
            **_probe_failure(_text(same[0]))}
    pod = (_first(pods, lambda p: p.get("waitingReason") == REASON_CRASHLOOP)
           or _first(restarted, lambda p: p.get("lastTerminated")))
    backoff = [e for e in events if e["reason"] == REASON_BACKOFF and CRASHLOOP_MARKER in _text(e).lower()]
    if pod is None:
        return "crashloop.exit_other", {**_replicas(status), **_event_params(backoff)}
    last = _last(pod)
    if pod.get("init"):
        reason = "crashloop.init_failure"
    elif last.get("reason") == REASON_START_ERROR:
        reason = "crashloop.start_error"
    else:
        reason = EXIT_CODE_REASONS.get(last.get("exitCode"), "crashloop.exit_other")
    return reason, {**_replicas(status), **_pod_params(pod)}


def _oom(status, pods, events, limits, change) -> tuple[str, Params]:
    pod = _first(pods, lambda p: _last(p).get("reason") == REASON_OOM_KILLED)
    if pod is None:
        # Event-only (no status read): nothing says which container or whether it had a limit.
        reason = "oom.node" if status is not None and not limits else "oom.limit"
        return reason, {**_replicas(status), **_event_params([e for e in events if e["reason"] in OOM_EVENT_REASONS])}
    limit = (limits or {}).get(pod.get("container"))
    params = {**_replicas(status), **_pod_params(pod), "limit": limit}
    return ("oom.limit" if limit else "oom.node"), params


def _probe(status, pods, events, limits, change) -> tuple[str, Params]:
    evs = [e for e in events if e["reason"] == REASON_UNHEALTHY]
    probe = probe_of(evs[0]) or PROBE_READINESS
    same = [e for e in evs if (probe_of(e) or PROBE_READINESS) == probe]
    params = {**_replicas(status), "pod": _pod_of(same[0]), "probe": probe, "count": sum(e["count"] for e in same),
              **_probe_failure(_text(same[0]))}
    return INSIGHT_KIND_PROBE_FAILURE + REASON_SEPARATOR + probe, params


def _scheduler_segment(text: str) -> str:
    lowered = text.lower()
    start = lowered.find(SCHEDULING_AVAILABLE_MARKER)
    segment = text[start + len(SCHEDULING_AVAILABLE_MARKER):] if start >= 0 else text
    end = segment.lower().find(SCHEDULING_PREEMPTION_MARKER)
    return _head(segment[:end] if end >= 0 else segment)


def scheduling_reason(text: str) -> str:
    """The segment the scheduler counts most nodes for; ties go to the earlier table row."""
    totals: Counter = Counter()
    for piece in re.split(SCHEDULING_PIECE_SPLIT, _scheduler_segment(text)):
        lowered = piece.lower()
        count = int(m.group(1)) if (m := re.match(SCHEDULING_COUNT_PATTERN, lowered)) else 1
        if hit := _first(SCHEDULING_SEGMENTS, lambda s: _matches(s[1], lowered)):
            totals[hit[0]] += count
    order = [reason for reason, _ in SCHEDULING_SEGMENTS]
    return max(totals, key=lambda r: (totals[r], -order.index(r))) if totals else SCHEDULING_OTHER


def _scheduling(status, pods, events, limits, change) -> tuple[str, Params]:
    evs = [e for e in events if e["reason"] == REASON_FAILED_SCHEDULING]
    text = _text(evs[0])
    end = text.lower().find(SCHEDULING_PREEMPTION_MARKER)
    message = re.sub(SCHEDULING_DRA_NOISE, "", text[:end] if end >= 0 else text)
    params: Params = {**_replicas(status), "pod": _pod_of(evs[0]), "message": _head(message).rstrip(",")}
    if status is not None and status.get(KEY_PENDING):
        params["pending"] = status[KEY_PENDING]
    return scheduling_reason(text), params


def _resource_pressure(status, pods, events, limits, change) -> tuple[str, Params]:
    preempted = _first(pods, lambda p: p.get(KEY_DISRUPTION) == REASON_PREEMPTION_BY_SCHEDULER)
    preempt_evs = [e for e in events if e["reason"] == REASON_PREEMPTED]
    if preempted or preempt_evs:
        params = {**_replicas(status), **_event_params(preempt_evs)}
        if preempted:
            params["pod"] = preempted["name"]
        return "resource_pressure.preempted", params
    evs = [e for e in events if e["reason"] == REASON_EVICTED or EVICTION_MARKER in _text(e).lower()]
    text = _text(evs[0])
    lowered = text.lower()
    reason = _first([r for r, markers in EVICTION_MARKERS if _matches(markers, lowered)], bool) or EVICTION_OTHER
    return reason, {**_replicas(status), **_event_params(evs[:1])}


def _rollout(status, pods, events, limits, change) -> tuple[str, Params]:
    params: Params = {**_replicas(status)}
    if status is not None:
        params["updated"] = status["updated"]
    created = [e for e in events if e["reason"] == REASON_FAILED_CREATE]
    for reason, markers in (("rollout_stuck.quota_exceeded", QUOTA_MARKERS),
                            ("rollout_stuck.admission_denied", ADMISSION_MARKERS)):
        if hit := _first(created, lambda e: _matches(markers, _text(e).lower())):
            return reason, {**params, "message": _head(re.sub(FAILED_CREATE_HEAD, "", _text(hit)))}
    stalled = status and _first(status.get("conditions") or [], lambda c: c.get("type") == CONDITION_PROGRESSING
                                and c.get("status") == CONDITION_FALSE
                                and c.get("reason") == REASON_PROGRESS_DEADLINE)
    return ("rollout_stuck.progress_deadline" if stalled else "rollout_stuck.incomplete"), params


def change_reason(field: str) -> str:
    if field in CHANGE_FIELD_REASONS:
        return CHANGE_FIELD_REASONS[field]
    if field.startswith(CHANGE_FIELD_RESOURCE_PREFIXES):
        return CHANGE_REASON_RESOURCES
    return CHANGE_REASON_OTHER


def _regression(status, pods, events, limits, change) -> tuple[str, Params]:
    return change_reason((change or {}).get("field") or ""), _replicas(status)


def _other(status, pods, events, limits, change) -> tuple[str, Params]:
    params = _replicas(status)
    if pod := _first(pods, lambda p: p.get("waitingReason") == REASON_CREATE_CONTAINER_CONFIG_ERROR):
        return "other.container_config_error", {**params, **_pod_params(pod),
                                                "message": _head(pod.get(KEY_WAITING_MESSAGE))}
    if pod := _first(pods, lambda p: p.get("waitingReason") in CREATE_CONTAINER_ERROR_REASONS):
        return "other.create_container_error", {**params, **_pod_params(pod),
                                                "message": _head(pod.get(KEY_WAITING_MESSAGE))}
    if mounts := [e for e in events if e["reason"] in VOLUME_EVENT_REASONS]:
        return "other.volume_mount", {**params, **_event_params(mounts[:1])}
    if not events:
        return "other.degraded", params
    counts = Counter()
    for e in events:
        counts[e["reason"]] += e["count"]
    reasons = WARNING_REASONS_SEPARATOR.join(r for r, _ in counts.most_common(WARNING_REASONS_MAX))
    return "other.warnings", {**params, "warnings": len(events), "reasons": reasons}


_CLASSIFIERS = {
    INSIGHT_KIND_IMAGE_PULL: _image_pull,
    INSIGHT_KIND_CRASHLOOP: _crashloop,
    INSIGHT_KIND_OOM: _oom,
    INSIGHT_KIND_PROBE_FAILURE: _probe,
    INSIGHT_KIND_SCHEDULING: _scheduling,
    INSIGHT_KIND_RESOURCE_PRESSURE: _resource_pressure,
    INSIGHT_KIND_ROLLOUT_STUCK: _rollout,
    INSIGHT_KIND_CONFIG_CHANGE_REGRESSION: _regression,
    INSIGHT_KIND_OTHER: _other,
}


def bounded(params: Params) -> dict[str, str]:
    """Only known keys with a value, as strings of at most MAX_INSIGHT_PARAM_LENGTH, at most MAX_INSIGHT_PARAMS."""
    out: dict[str, str] = {}
    for key in INCIDENT_PARAM_KEYS:
        value = params.get(key)
        if value is None or value == "":
            continue
        text = str(value)
        if len(text) > MAX_INSIGHT_PARAM_LENGTH:
            text = text[:MAX_INSIGHT_PARAM_LENGTH - len(PARAM_ELLIPSIS)] + PARAM_ELLIPSIS
        out[key] = text
    return dict(list(out.items())[:MAX_INSIGHT_PARAMS])


def classify(kind: str, status: dict | None, pods: list[dict], events: list[dict], limits: dict | None,
             change: dict | None) -> tuple[str, dict[str, str]]:
    """(reason, params) of one incident candidate whose kind the rules decided; `events` are the workload's own."""
    reason, params = _CLASSIFIERS[kind](status, pods, events, limits or {}, change)
    if change:
        params.update(generation=change["generation"], change=change["change"])
    return reason, bounded(params)


# ---- rendering -------------------------------------------------------------------------------------------------
def placeholders(template: str) -> set[str]:
    return {name for _, name, _, _ in string.Formatter().parse(template) if name}


def pick(templates: tuple[str, ...], values: dict[str, str]) -> str:
    """The first template whose placeholders are all known values; '' when none is."""
    return next((t for t in templates if placeholders(t) <= values.keys()), "")


def fill(templates: tuple[str, ...], values: dict[str, str]) -> str:
    return pick(templates, values).format_map(values)


def _plurals(params: dict[str, str]) -> dict[str, str]:
    if (restarts := params.get("restarts")) is None:
        return dict(params)
    form = int(str(restarts) != RESTARTS_SINGULAR)
    return {**params, **{key: forms[form] for key, forms in RESTART_PLURALS.items()}}


def _derived(params: dict[str, str]) -> dict[str, str]:
    values = _plurals(params)
    if failure := params.get("failure"):
        detail = params.get("message", "")
        if m := re.match(PROBE_MESSAGE_PATTERN, detail, re.IGNORECASE):
            detail = detail[m.end():]
        values["failureText"] = fill(PROBE_FAILURE_TEXT[failure], {**params, "detail": detail} if detail else params)
    return values


def what(reason: str, params: dict[str, str]) -> str:
    """The reason's title phrase; '' for an unknown reason (a deep-mode insight carries none)."""
    if reason not in INCIDENT_TEXT:
        return ""
    return fill((INCIDENT_TEXT[reason][0],), params) or INCIDENT_TEXT[reason][0].split(" {", 1)[0]


def _state(status: dict | None) -> str:
    if not status or not status["desired"] or status["ready"] >= status["desired"]:
        return ""
    return STATE_DOWN if status["ready"] == 0 else STATE_DEGRADED


def render_incident(reason: str, params: dict[str, str], workload: str, status: dict | None,
                    change_sentence: str) -> tuple[str, str]:
    """(title, summary): '{workload} {state}: {what}' and '{impact} {detail}{change}'."""
    phrase = what(reason, params)
    values = {**_derived(params), "workload": workload, "whatSentence": phrase[:1].upper() + phrase[1:]}
    state = _state(status)
    title = (TITLE_WITH_STATE if state else TITLE_NO_STATE).format(workload=workload, state=state, what=phrase)
    template = pick((INCIDENT_TEXT[reason][1], *INCIDENT_DETAIL_VARIANTS.get(reason, ()), *INCIDENT_DETAIL_FALLBACKS),
                    values)
    impact = ""
    # A detail that states the replica counts itself makes the impact sentence a repeat.
    if status and status["desired"] and "ready" not in placeholders(template):
        impact = (IMPACT_DOWN if status["ready"] == 0 else IMPACT_PARTIAL).format(
            ready=status["ready"], desired=status["desired"], workload=workload)
    summary = " ".join(part for part in (impact, template.format_map(values)) if part) + change_sentence
    return title[:MAX_INSIGHT_TITLE_LENGTH], summary.strip()[:MAX_INSIGHT_SUMMARY_LENGTH]


def render_recommendation(reason: str, params: dict[str, str]) -> tuple[str, str]:
    """(title, summary) of a recommendation: exact template text, never narrated."""
    values = _plurals(params)
    if mode := params.get("mode"):
        values["modeText"] = ROOT_MODE_TEXT[mode]
    title, summary = RECOMMENDATION_TEXT[reason]
    return title.format_map(values)[:MAX_INSIGHT_TITLE_LENGTH], summary.format_map(values)[:MAX_INSIGHT_SUMMARY_LENGTH]
