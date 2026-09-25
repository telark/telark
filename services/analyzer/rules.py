"""Fast mode's detector: deterministic candidate insights from the gathered tool results.

Pure: reads the overview and history payloads, run.status_cache and run.events_cache
(exactly what the tools returned) and never calls anything. Code decides kind,
severity, confidence and evidence; the narration may only rephrase title/summary.
"""

from __future__ import annotations

import re
from datetime import datetime

from constants import (
    CHANGE_CORRELATION_WINDOW_S,
    CHANGE_PARAM_TEMPLATE,
    CONDITION_FALSE,
    CONDITION_PROGRESSING,
    CONFIDENCE_HIGH,
    CONFIDENCE_LOW,
    CONFIDENCE_MEDIUM,
    CORRELATED_CHANGE_CLASSES,
    CORRELATION_DETAIL_TEMPLATE,
    CORRELATION_SUFFIX_TEMPLATE,
    CORRELATION_TEMPLATE,
    CRASHLOOP_MARKER,
    EVICTION_MARKER,
    EVIDENCE_TYPE_CHANGE,
    EVIDENCE_TYPE_EVENT,
    EVIDENCE_TYPE_WORKLOAD,
    FACT_TEMPLATE,
    FACT_WHAT,
    FAST_STATUS_MAX,
    IMAGE_PULL_MARKERS,
    IMAGE_PULL_WAITING_REASONS,
    INSIGHT_KIND_CONFIG_CHANGE_REGRESSION,
    INSIGHT_KIND_CRASHLOOP,
    INSIGHT_KIND_IMAGE_PULL,
    INSIGHT_KIND_OOM,
    INSIGHT_KIND_OTHER,
    INSIGHT_KIND_PROBE_FAILURE,
    INSIGHT_KIND_RESOURCE_PRESSURE,
    INSIGHT_KIND_ROLLOUT_STUCK,
    INSIGHT_KIND_SCHEDULING,
    INSIGHT_SEVERITY_CRITICAL,
    INSIGHT_SEVERITY_WARNING,
    MAX_EVIDENCE_PER_INSIGHT,
    MAX_INSIGHTS_PER_RUN,
    MIN_REFS_FOR_CONFIDENCE,
    NONE_VALUE,
    OOM_EVENT_REASONS,
    REASON_BACKOFF,
    REASON_CRASHLOOP,
    REASON_EVICTED,
    REASON_FAILED,
    REASON_FAILED_CREATE,
    REASON_FAILED_SCHEDULING,
    REASON_OOM_KILLED,
    REASON_PREEMPTED,
    REASON_PREEMPTION_BY_SCHEDULER,
    REASON_UNHEALTHY,
    REF_EVENT,
    REF_GENERATION,
    REF_WORKLOAD,
    RESTARTING_PROBES,
    S_PER_MINUTE,
    SEVERITY_RANK,
    SUBJECT_SEPARATOR,
    ADMISSION_MARKERS,
    QUOTA_MARKERS,
    CREATE_CONTAINER_ERROR_REASONS,
    REASON_CREATE_CONTAINER_CONFIG_ERROR,
    VOLUME_EVENT_REASONS,
)
import messages
from models import Candidate, EvidenceRef, Run
from tools.app_tools import KEY_CHANGES
from tools.k8s_tools import (
    KEY_DISRUPTION,
    KEY_FULL_MESSAGE,
    KEY_LIMITS,
    KEY_NAMESPACE,
    KEY_PODS,
    stale_pod_event,
    workload_matchers,
)

# (kind, severity, confidence, the events it rests on); None when the rule does not fire.
Match = tuple[str, str, str, list[dict]] | None


def _owned(event: dict, namespace: str, matchers: list) -> bool:
    kind, _, name = event["object"].partition(SUBJECT_SEPARATOR)
    return event[KEY_NAMESPACE] == namespace and any(kind == k and pattern.match(name) for k, pattern in matchers)


def _text(event: dict) -> str:
    return (event.get(KEY_FULL_MESSAGE) or event["message"]).lower()


def _first_pod(pods: list[dict], test) -> dict | None:
    return next((p for p in pods if test(p)), None)


def _oom(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    pod = _first_pod(pods, lambda p: (p.get("lastTerminated") or {}).get("reason") == REASON_OOM_KILLED)
    evs = [e for e in events if e["reason"] in OOM_EVENT_REASONS]
    if not (pod or evs):
        return None
    return INSIGHT_KIND_OOM, INSIGHT_SEVERITY_CRITICAL, CONFIDENCE_HIGH, evs


def _image_pull(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    pod = _first_pod(pods, lambda p: p.get("waitingReason") in IMAGE_PULL_WAITING_REASONS)
    evs = [e for e in events if e["reason"] in (REASON_FAILED, REASON_BACKOFF)
           and any(m in _text(e) for m in IMAGE_PULL_MARKERS)]
    if not (pod or evs):
        return None
    return INSIGHT_KIND_IMAGE_PULL, INSIGHT_SEVERITY_CRITICAL, CONFIDENCE_HIGH, evs


def _crashloop(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    """A restart loop; a liveness or startup probe that restarted a container is one too (crashloop.probe_kill)."""
    pod = _first_pod(pods, lambda p: p.get("waitingReason") == REASON_CRASHLOOP)
    evs = [e for e in events if e["reason"] == REASON_BACKOFF and CRASHLOOP_MARKER in _text(e)]
    restarted = any((p.get("restarts") or 0) > 0 for p in pods)
    probe_evs = [e for e in events if e["reason"] == REASON_UNHEALTHY and messages.probe_of(e) in RESTARTING_PROBES]
    if not (pod or evs or (restarted and probe_evs)):
        return None
    return INSIGHT_KIND_CRASHLOOP, INSIGHT_SEVERITY_CRITICAL, CONFIDENCE_HIGH, evs + (probe_evs if restarted else [])


def _scheduling(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    evs = [e for e in events if e["reason"] == REASON_FAILED_SCHEDULING]
    if not evs:
        return None
    short = status is not None and status["ready"] < status["desired"]
    severity = INSIGHT_SEVERITY_CRITICAL if short else INSIGHT_SEVERITY_WARNING
    return INSIGHT_KIND_SCHEDULING, severity, CONFIDENCE_HIGH, evs


def _probe_failure(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    evs = [e for e in events if e["reason"] == REASON_UNHEALTHY]
    if not evs:
        return None
    none_ready = status is not None and status["ready"] == 0
    severity = INSIGHT_SEVERITY_CRITICAL if none_ready else INSIGHT_SEVERITY_WARNING
    confidence = CONFIDENCE_HIGH if len(evs) >= MIN_REFS_FOR_CONFIDENCE else CONFIDENCE_MEDIUM
    return INSIGHT_KIND_PROBE_FAILURE, severity, confidence, evs


def _resource_pressure(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    # The scheduler's Preempted event is type Normal (unseen by warningsOnly reads): the pod condition shows it.
    evs = [e for e in events if e["reason"] in (REASON_EVICTED, REASON_PREEMPTED) or EVICTION_MARKER in _text(e)]
    preempted = _first_pod(pods, lambda p: p.get(KEY_DISRUPTION) == REASON_PREEMPTION_BY_SCHEDULER)
    if not (evs or preempted):
        return None
    return INSIGHT_KIND_RESOURCE_PRESSURE, INSIGHT_SEVERITY_WARNING, CONFIDENCE_HIGH, evs


def _rollout_stuck(status: dict | None, pods: list[dict], events: list[dict]) -> Match:
    """A stalled or lagging rollout, or new pods refused by a quota or an admission policy (no status needed)."""
    refused = [e for e in events if e["reason"] == REASON_FAILED_CREATE
               and any(re.search(m, _text(e)) for m in QUOTA_MARKERS + ADMISSION_MARKERS)]
    if refused:
        return INSIGHT_KIND_ROLLOUT_STUCK, INSIGHT_SEVERITY_WARNING, CONFIDENCE_MEDIUM, refused
    if status is None:
        return None
    # Pods that cannot create their container or mount a volume name the cause: the 'other' path reports it,
    # even once the rollout passes its progress deadline.
    creating = (REASON_CREATE_CONTAINER_CONFIG_ERROR, *CREATE_CONTAINER_ERROR_REASONS)
    if _first_pod(pods, lambda p: p.get("waitingReason") in creating) or any(
            e["reason"] in VOLUME_EVENT_REASONS for e in events):
        return None
    stalled = next((c for c in status["conditions"]
                    if c.get("type") == CONDITION_PROGRESSING and c.get("status") == CONDITION_FALSE), None)
    lagging = status["updated"] < status["desired"] and status["ready"] < status["desired"]
    if not (stalled or lagging):
        return None
    return INSIGHT_KIND_ROLLOUT_STUCK, INSIGHT_SEVERITY_WARNING, CONFIDENCE_MEDIUM, []


# Precedence order: the first match names the card, every match adds its evidence.
_SYMPTOM_RULES = (_oom, _image_pull, _crashloop, _scheduling, _probe_failure, _resource_pressure, _rollout_stuck)


def _recent_change(history: dict, now: datetime) -> dict | None:
    """The newest entry when it is an incident or a rollout/config/resources change inside the window:
    {generation, fact, field, change}."""
    entries = history.get(KEY_CHANGES) or []
    if not entries:
        return None
    entry = entries[0]
    if not (entry.get("isIncident") or entry.get("changeClass") in CORRELATED_CHANGE_CLASSES):
        return None
    try:
        age = (now - datetime.fromisoformat(entry.get("detectedAt") or "")).total_seconds()
    except ValueError:
        return None
    if age > CHANGE_CORRELATION_WINDOW_S:
        return None
    generation = entry.get("generation")
    change = {"generation": generation, "field": "", "change": "",
              "fact": CORRELATION_TEMPLATE.format(minutes=max(0, int(age // S_PER_MINUTE)), generation=generation)}
    if changes := entry.get(KEY_CHANGES):
        first = changes[0]
        values = dict(field=first.get("field"), old=first.get("oldValue") or NONE_VALUE,
                      new=first.get("newValue") or NONE_VALUE)
        change.update(field=first.get("field") or "", change=CHANGE_PARAM_TEMPLATE.format(**values))
        change["fact"] += CORRELATION_DETAIL_TEMPLATE.format(**values)
    return change


def _candidate(run: Run, key: tuple[str, str], status: dict | None, evs: list[dict], matches: list,
               change: dict | None) -> Candidate:
    kind, severity, confidence, _events = matches[0]
    correlated = change if kind != INSIGHT_KIND_OTHER else None
    namespace, subject = key
    kind_name, _, name = subject.partition(SUBJECT_SEPARATOR)
    refs = [EvidenceRef(type=EVIDENCE_TYPE_WORKLOAD, ref=REF_WORKLOAD.format(kind=kind_name, name=name))]
    if correlated:
        refs.append(EvidenceRef(type=EVIDENCE_TYPE_CHANGE, ref=REF_GENERATION.format(generation=correlated["generation"])))
    refs += [EvidenceRef(type=EVIDENCE_TYPE_EVENT, ref=REF_EVENT.format(**e)) for m in matches for e in m[3]]
    evidence = list({r.ref: r for r in refs if r.ref in run.refs}.values())[:MAX_EVIDENCE_PER_INSIGHT]

    pods = status[KEY_PODS] if status else []
    reason, params = messages.classify(kind, status, pods, evs, (status or {}).get(KEY_LIMITS), correlated)
    params = messages.bounded({"workload": name, "namespace": namespace, **params})
    sentence = CORRELATION_SUFFIX_TEMPLATE.format(correlated["fact"]) if correlated else ""
    title, summary = messages.render_incident(reason, params, name, status, sentence)
    facts = [FACT_TEMPLATE.format(FACT_WHAT, messages.what(reason, params))]
    facts += [FACT_TEMPLATE.format(k, v) for k, v in params.items()]
    if correlated:
        facts.append(correlated["fact"])
    return Candidate(subject=subject, kind=kind, severity=severity, confidence=confidence, evidence=evidence,
                     title=title, summary=summary, facts=facts, reason=reason, params=params)


def evaluate(run: Run, overview: dict, history: dict, statuses: dict[str, dict], events: list[dict],
             now: datetime) -> list[Candidate]:
    """At most MAX_INSIGHTS_PER_RUN candidates, one per workload, most severe first."""
    change = _recent_change(history, now)
    health = overview.get("health") or {}
    app_degraded = (health.get("ready") or 0) < (health.get("total") or 0)
    out = []
    for key, (kind, name, namespace) in run.workloads.items():
        status = statuses.get(key)
        matchers = workload_matchers(kind, name)
        items = run.pods_cache.get(key)
        evs = [e for e in events if _owned(e, namespace, matchers) and not stale_pod_event(status, items, e, now)]
        # Without its status only a lone workload can be the one the app's health is about.
        degraded = status["ready"] < status["desired"] if status else app_degraded and len(run.workloads) == 1
        matches = [m for rule in _SYMPTOM_RULES if (m := rule(status, status[KEY_PODS] if status else [], evs))]
        if not matches and degraded and change:
            matches = [(INSIGHT_KIND_CONFIG_CHANGE_REGRESSION, INSIGHT_SEVERITY_WARNING, CONFIDENCE_MEDIUM, [])]
        if not matches and (degraded or evs):
            matches = [(INSIGHT_KIND_OTHER, INSIGHT_SEVERITY_WARNING, CONFIDENCE_LOW, evs)]
        if matches:
            out.append(_candidate(run, key, status, evs, matches, change))
    out.sort(key=lambda c: SEVERITY_RANK[c.severity], reverse=True)
    return out[:MAX_INSIGHTS_PER_RUN]


def select_status_subjects(run: Run, events: list[dict]) -> list[tuple[str, str]]:
    """The workload keys worth a status read: those named by warning events first, then overview order."""
    matchers = {key: workload_matchers(kind, name) for key, (kind, name, _ns) in run.workloads.items()}
    named = [key for e in events for key, m in matchers.items() if _owned(e, key[0], m)]
    return list(dict.fromkeys(named + list(run.workloads)))[:FAST_STATUS_MAX]
