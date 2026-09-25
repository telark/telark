"""Insight document store and the code-owned insight lifecycle.

One JSON document per app at analyzer:{ns}:{name}, indexed in the analyzer:index
ZSET for discovery's Insights page. Discovery's read route serves the same key, so the
document stays readable while the analyzer is off or absent. The model only proposes
incidents and the rules only propose recommendations: ids, statuses, timestamps,
triage and every resolve are decided here.
"""

from __future__ import annotations

import asyncio
import hashlib
import json
from collections.abc import Callable
from datetime import datetime, timedelta

from pydantic import ValidationError

from config import ANALYZER_AUTO_COOLDOWN_SEC, ANALYZER_MANUAL_COOLDOWN_SEC
from constants import (
    CONFIDENCE_LOW,
    CONSUMER_GROUP,
    COOLDOWN_VALUE,
    DOCUMENT_TTL_S,
    INFLIGHT_TTL_S,
    INDEX_KEY,
    INDEX_SCORE_MIN,
    INSIGHT_CATEGORY_INCIDENT,
    INSIGHT_CATEGORY_RECOMMENDATION,
    INSIGHT_ID_LENGTH,
    INSIGHT_ID_TEMPLATE,
    INSIGHT_KIND_OTHER,
    INSIGHT_STATUS_OPEN,
    INSIGHT_STATUS_RESOLVED,
    INSIGHT_STATUS_UPDATED,
    JSON_COMPACT_SEPARATORS,
    LEGACY_INSIGHT_ID_TEMPLATE,
    MAX_INSIGHT_SUMMARY_LENGTH,
    MAX_INSIGHT_TITLE_LENGTH,
    MAX_RECOMMENDATIONS_PER_APP,
    MIN_REFS_FOR_CONFIDENCE,
    MS_PER_S,
    RECOMMENDATION_ID_TEMPLATE,
    RESOLVED_RETENTION_S,
    RFC3339_FORMAT,
    RUN_STATUS_FAILED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_RUNNING,
    SEVERITY_RANK,
    STREAM_JOBS,
    SUBJECT_SEPARATOR,
    SUBJECT_TEMPLATE,
    TRIAGE_ACTION_DISMISS,
    TRIAGE_ACTION_REOPEN,
    TRIAGE_ERROR_INVALID,
    TRIAGE_ERROR_NOT_FOUND,
    TRIAGE_STATE_ACKNOWLEDGED,
    TRIAGE_STATE_DISMISSED,
    TRIGGER_MANUAL,
    VOLATILE_PARAMS,
)
from helpers import app_ref, cooldown_auto_key, cooldown_manual_key, document_key, epoch_ms, inflight_key
from messages import render_recommendation
from models import AppInsights, Emitted, Insight, InsightTriage, LastRun, MergeStats, RecStats, Run, to_json
from recommendations import card_key
from tools.k8s_tools import KEY_NAMESPACE, KEY_PODS, stale_pod_event, workload_matchers

_ACTIVE = (INSIGHT_STATUS_OPEN, INSIGHT_STATUS_UPDATED)
_PENDING = (RUN_STATUS_QUEUED, RUN_STATUS_RUNNING)


class InsightStore:
    def __init__(self, r) -> None:
        self._r = r
        # One store per process, built once by main: a module-level Lock would bind to the first event loop.
        self._lock = asyncio.Lock()

    async def get(self, namespace: str, name: str) -> AppInsights:
        """Missing, undecodable or legacy documents read as empty (discovery cache.go rule)."""
        raw = await self._r.get(document_key(namespace, name))
        if not raw:
            return AppInsights()
        try:
            doc = AppInsights.model_validate_json(raw)
        except ValidationError:
            return AppInsights()
        if doc.version == 0 and not doc.lastRun.status:
            return AppInsights()
        return doc

    async def put(self, namespace: str, name: str, doc: AppInsights) -> None:
        """The document, then its index entry: discovery re-reads a member when its score moves, so the
        score may move only once the new document is in place (a pipeline runs its commands in order)."""
        pipe = self._r.pipeline(transaction=False)
        pipe.set(document_key(namespace, name), to_json(doc), ex=DOCUMENT_TTL_S)
        pipe.zadd(INDEX_KEY, {app_ref(namespace, name): epoch_ms()})
        await pipe.execute()

    async def delete(self, namespace: str, name: str) -> None:
        pipe = self._r.pipeline(transaction=False)
        pipe.delete(document_key(namespace, name))
        pipe.zrem(INDEX_KEY, app_ref(namespace, name))
        await pipe.execute()

    async def gc_index(self, now_ms: int) -> int:
        """Drop index members older than the document TTL: their document has expired."""
        return await self._r.zremrangebyscore(INDEX_KEY, INDEX_SCORE_MIN, now_ms - DOCUMENT_TTL_S * MS_PER_S)

    async def update(
        self, namespace: str, name: str, fn: Callable[[AppInsights], bool]
    ) -> tuple[AppInsights, bool]:
        """The only document write path: read, apply `fn`, write only when it reports a change."""
        # ponytail: in-process lock, correct because the analyzer is one replica on one event loop; WATCH/MULTI if replicas > 1.
        async with self._lock:
            doc = await self.get(namespace, name)
            changed = fn(doc)
            if changed:
                await self.put(namespace, name, doc)
            return doc, changed

    async def acquire_inflight(self, namespace: str, name: str, run_id: str) -> tuple[bool, str]:
        """(True, run_id) when taken, else (False, the holder's run id)."""
        key = inflight_key(namespace, name)
        if await self._r.set(key, run_id, nx=True, ex=INFLIGHT_TTL_S):
            return True, run_id
        return False, await self._r.get(key) or ""

    async def release_inflight(self, namespace: str, name: str, run_id: str) -> None:
        # ponytail: GET then DEL is safe because the single worker task is the only inflight writer.
        key = inflight_key(namespace, name)
        if await self._r.get(key) == run_id:
            await self._r.delete(key)

    async def cooldown_manual(self, namespace: str, name: str) -> bool:
        key = cooldown_manual_key(namespace, name)
        return bool(await self._r.set(key, COOLDOWN_VALUE, nx=True, ex=ANALYZER_MANUAL_COOLDOWN_SEC))

    async def cooldown_auto(self, namespace: str, name: str) -> bool:
        key = cooldown_auto_key(namespace, name)
        return bool(await self._r.set(key, COOLDOWN_VALUE, nx=True, ex=ANALYZER_AUTO_COOLDOWN_SEC))

    async def queue_len(self) -> int:
        return await self._r.xlen(STREAM_JOBS)

    async def ack_job(self, msg_id: str) -> None:
        """The worker's only acknowledgement: XACK then XDEL, so XLEN = undelivered + pending backlog."""
        await self._r.xack(STREAM_JOBS, CONSUMER_GROUP, msg_id)
        await self._r.xdel(STREAM_JOBS, msg_id)


def insight_id(namespace: str, name: str, subject: str, workload_namespace: str) -> str:
    """One card per workload (namespace, kind and name): kind is an attribute, never part of the id."""
    key = INSIGHT_ID_TEMPLATE.format(namespace=namespace, name=name, subject=subject,
                                     workload_namespace=workload_namespace)
    return hashlib.sha256(key.encode()).hexdigest()[:INSIGHT_ID_LENGTH]


def _legacy_card(cards: dict[str, Insight], namespace: str, name: str, subject: str,
                 workload_namespace: str) -> Insight | None:
    """This workload's card under the id without its namespace; a card naming another namespace is not it."""
    key = LEGACY_INSIGHT_ID_TEMPLATE.format(namespace=namespace, name=name, subject=subject)
    card = cards.get(hashlib.sha256(key.encode()).hexdigest()[:INSIGHT_ID_LENGTH])
    if card is None or card.params.get("namespace", workload_namespace) != workload_namespace:
        return None
    return card


def _canonical(subject: str) -> str:
    kind, _, name = subject.partition(SUBJECT_SEPARATOR)
    return SUBJECT_TEMPLATE.format(kind=kind.lower(), name=name)


def validate(emitted: list[Emitted], run: Run) -> list[Emitted]:
    """Keep what this run's tools back: a known workload, known refs, one insight per workload. Every kept insight
    names its workload's namespace in params (deep mode's model names none: the first app namespace with it)."""
    kept: dict[tuple[str, str], Emitted] = {}
    for e in emitted:
        subject = _canonical(e.subject)
        key = run.bind(subject, e.params.get("namespace", ""))
        if key is None or key in run.gone:
            continue
        evidence = list({ref.ref: ref for ref in e.evidence if ref.ref in run.refs}.values())
        confidence = e.confidence
        if not evidence:
            if e.kind != INSIGHT_KIND_OTHER:
                continue
            confidence = CONFIDENCE_LOW
        candidate = e.model_copy(update={
            "subject": subject,
            "evidence": evidence,
            "confidence": confidence,
            "title": e.title[:MAX_INSIGHT_TITLE_LENGTH],
            "summary": e.summary[:MAX_INSIGHT_SUMMARY_LENGTH],
            "params": {**e.params, "namespace": key[0]},
        })
        held = kept.get(key)
        if held is None or SEVERITY_RANK[candidate.severity] > SEVERITY_RANK[held.severity]:
            kept[key] = candidate
    return list(kept.values())


def merge(doc: AppInsights, valid: list[Emitted], now: str, truncated: bool, namespace: str, name: str) -> MergeStats:
    """Apply validated insights; anything not re-emitted is left untouched (omission never resolves)."""
    stats = MergeStats()
    cards = {card.id: card for card in doc.insights}
    for e in valid:
        workload_namespace = e.params.get("namespace", namespace)
        iid = insight_id(namespace, name, e.subject, workload_namespace)
        weak = truncated or len(e.evidence) < MIN_REFS_FOR_CONFIDENCE
        fields = {
            "kind": e.kind,
            "category": INSIGHT_CATEGORY_INCIDENT,
            "reason": e.reason,
            "params": dict(e.params),
            "title": e.title,
            "summary": e.summary,
            "severity": e.severity,
            "confidence": CONFIDENCE_LOW if weak else e.confidence,
            "evidence": list(e.evidence),
            "lastSeenAt": now,
        }
        card = cards.get(iid) or _legacy_card(cards, namespace, name, e.subject, workload_namespace)
        if card is None:
            card = Insight(id=iid, subject=e.subject, status=INSIGHT_STATUS_OPEN, firstSeenAt=now, runs=1, **fields)
            doc.insights.append(card)
            cards[iid] = card
            stats.created.append(iid)
        else:
            card.id, cards[iid] = iid, card
            if card.status == INSIGHT_STATUS_RESOLVED:
                card.status, card.resolvedAt = INSIGHT_STATUS_OPEN, ""
                stats.reopened.append(iid)
            else:
                card.status = INSIGHT_STATUS_UPDATED
                stats.updated.append(iid)
            card.runs += 1
            for field, value in fields.items():
                setattr(card, field, value)
        stats.touched.add(iid)
    return stats


def narrate(doc: AppInsights, ids: list[str], texts: list[tuple[str, str]]) -> list[str]:
    """Rephrase cards: title and summary only, both non-empty, never any code-owned field; returns the ids set."""
    cards = {card.id: card for card in doc.insights}
    rewritten = []
    for iid, (title, summary) in zip(ids, texts):
        card = cards.get(iid)
        title, summary = title.strip(), summary.strip()
        if card is None or not (title and summary):
            continue
        card.title, card.summary = title[:MAX_INSIGHT_TITLE_LENGTH], summary[:MAX_INSIGHT_SUMMARY_LENGTH]
        rewritten.append(iid)
    return rewritten


def _resolve(card: Insight, now: str) -> None:
    card.status, card.resolvedAt, card.triage = INSIGHT_STATUS_RESOLVED, now, None


def _resolve_where(doc: AppInsights, now: str, recovered: Callable[[Insight], bool]) -> list[str]:
    """Incident cards only: recommendations follow their own lifecycle (merge_recommendations)."""
    resolved = []
    for card in doc.insights:
        if card.category != INSIGHT_CATEGORY_RECOMMENDATION and card.status in _ACTIVE and recovered(card):
            _resolve(card, now)
            resolved.append(card.id)
    return resolved


def resolve_all(doc: AppInsights, now: str) -> list[str]:
    """Recovery job: every open or updated incident resolves, no model call."""
    return _resolve_where(doc, now, lambda card: True)


def _observed_recovered(card: Insight, run: Run, now: datetime) -> bool:
    namespace = card.params.get("namespace", "")
    # Excluded after the card was written: no run reads that namespace again, so it would stay open forever.
    if namespace in run.excluded:
        return True
    key = run.bind(card.subject, namespace)
    if key in run.gone:
        return True
    status = run.status_cache.get(key)
    if status is None or status["ready"] != status["desired"]:
        return False
    if any(pod["waitingReason"] for pod in status[KEY_PODS]):
        return False
    kind, name, namespace = run.workloads[key]
    matchers = workload_matchers(kind, name)
    items = run.pods_cache.get(key)
    for event in run.events_cache:
        # Both sides use RFC3339_FORMAT (UTC, fixed width): string order is time order.
        if (event[KEY_NAMESPACE] != namespace or event["last"] <= card.lastSeenAt
                or stale_pod_event(status, items, event, now)):
            continue
        # Cached events carry no type (warningsOnly may be off): any newer event of the workload blocks.
        obj_kind, _, obj_name = event["object"].partition(SUBJECT_SEPARATOR)
        if any(obj_kind == k and pattern.match(obj_name) for k, pattern in matchers):
            return False
    return True


def resolve_observed(doc: AppInsights, run: Run, now: str, skip: set[str]) -> list[str]:
    """After an untruncated run: resolve cards whose workload this run saw healthy, except those in `skip`."""
    at = datetime.fromisoformat(now)
    return _resolve_where(doc, now, lambda card: card.id not in skip and _observed_recovered(card, run, at))


def prune(doc: AppInsights, now: str) -> None:
    """Drop resolved insights whose resolvedAt is older than RESOLVED_RETENTION_S."""
    cutoff = (datetime.fromisoformat(now) - timedelta(seconds=RESOLVED_RETENTION_S)).strftime(RFC3339_FORMAT)
    doc.insights = [c for c in doc.insights if not (c.status == INSIGHT_STATUS_RESOLVED and c.resolvedAt < cutoff)]


def stamp_run(doc: AppInsights, last_run: LastRun) -> None:
    doc.lastRun = last_run
    doc.version += 1


def stamp_review(doc: AppInsights, now: str) -> None:
    """A review's write: never lastRun, always a new version (the UI refetches on a higher one, and discovery
    skips a version-0 document without a lastRun as legacy)."""
    doc.lastReviewAt = now
    doc.version += 1


# ---- recommendations (D14) and triage (D15) -------------------------------------------------------------------
def recommendation_id(namespace: str, name: str, key: str, reason: str, workload_namespace: str) -> str:
    raw = RECOMMENDATION_ID_TEMPLATE.format(namespace=namespace, name=name, subject=key, reason=reason,
                                            workload_namespace=workload_namespace)
    return hashlib.sha256(raw.encode()).hexdigest()[:INSIGHT_ID_LENGTH]


def fingerprint(params: dict[str, str], severity: str, confidence: str) -> str:
    """The facts of a card: a change makes it 'updated'. Measurements that move with every sample do not count."""
    stable = {k: v for k, v in params.items() if k not in VOLATILE_PARAMS}
    raw = json.dumps([sorted(stable.items()), severity, confidence], separators=JSON_COMPACT_SEPARATORS)
    return hashlib.sha256(raw.encode()).hexdigest()


def _card_fingerprint(card: Insight) -> str:
    return fingerprint(card.params, card.severity, card.confidence)


def _stays_dismissed(card: Insight | None, f) -> bool:
    return (card is not None and card.status in _ACTIVE and card.triage is not None
            and card.triage.state == TRIAGE_STATE_DISMISSED
            and _card_fingerprint(card) == fingerprint(f.params, f.severity, f.confidence))


def merge_recommendations(doc: AppInsights, findings: list, evaluated: set[tuple[str, str, str]], now: str,
                          namespace: str, name: str) -> RecStats:
    """Apply one review: create, refresh, update, reopen, and resolve what a complete read no longer finds.

    `findings` come most severe first; a key that was not evaluated leaves its card untouched. The cap keeps the
    MAX_RECOMMENDATIONS_PER_APP most severe; a dismissed card is kept up to date and takes no place under it."""
    stats = RecStats()
    cards = {c.id: c for c in doc.insights if c.category == INSIGHT_CATEGORY_RECOMMENDATION}
    ids = [recommendation_id(namespace, name, f.key, f.reason, f.namespace) for f in findings]
    found = set(ids)
    ranked =[iid for iid, f in zip(ids, findings) if not _stays_dismissed(cards.get(iid), f)]
    # Past the cap a card is removed, not resolved: it is still true, and comes back once there is room.
    overflow = set(ranked[MAX_RECOMMENDATIONS_PER_APP:])
    doc.insights = [c for c in doc.insights if c.id not in overflow]
    for iid, f in zip(ids, findings):
        if iid in overflow:
            continue
        title, summary = render_recommendation(f.reason, f.params)
        fields = {"kind": f.kind, "subject": f.subject, "title": title, "summary": summary, "severity": f.severity,
                  "confidence": f.confidence, "evidence": list(f.evidence), "params": dict(f.params),
                  "lastSeenAt": now}
        card = cards.get(iid)
        if card is not None and card.status in _ACTIVE:
            changed = _card_fingerprint(card) != fingerprint(f.params, f.severity, f.confidence)
            for key, value in fields.items():
                setattr(card, key, value)
            if changed:
                card.status, card.triage = INSIGHT_STATUS_UPDATED, None
                card.runs += 1
                stats.updated.append(iid)
            continue
        if card is None:
            card = Insight(id=iid, status=INSIGHT_STATUS_OPEN, firstSeenAt=now, runs=1,
                           category=INSIGHT_CATEGORY_RECOMMENDATION, reason=f.reason, **fields)
            doc.insights.append(card)
            stats.created.append(iid)
        else:
            for key, value in fields.items():
                setattr(card, key, value)
            card.status, card.resolvedAt, card.triage = INSIGHT_STATUS_OPEN, "", None
            card.runs += 1
            stats.reopened.append(iid)
    for card in cards.values():
        key = (card.reason, card.params.get("namespace", ""), card_key(card.reason, card.subject, card.params))
        if card.status in _ACTIVE and card.id not in found and key in evaluated:
            _resolve(card, now)
            stats.resolved.append(card.id)
    return stats


class TriageError(Exception):
    """A refused triage; `code` is the API error code."""

    def __init__(self, code: str) -> None:
        super().__init__(code)
        self.code = code


def apply_triage(doc: AppInsights, iid: str, action: str, user_id: str, now: str) -> tuple[Insight, bool]:
    """(the card, whether it changed): a change bumps the version (the UI refetches on a higher one); a triage
    that changes nothing is not written."""
    card = next((c for c in doc.insights if c.id == iid), None)
    if card is None:
        raise TriageError(TRIAGE_ERROR_NOT_FOUND)
    if action == TRIAGE_ACTION_REOPEN:
        if card.triage is None:
            return card, False
        card.triage = None
    else:
        if card.status == INSIGHT_STATUS_RESOLVED or (
                action == TRIAGE_ACTION_DISMISS and card.category != INSIGHT_CATEGORY_RECOMMENDATION):
            raise TriageError(TRIAGE_ERROR_INVALID)
        state = TRIAGE_STATE_DISMISSED if action == TRIAGE_ACTION_DISMISS else TRIAGE_STATE_ACKNOWLEDGED
        if card.triage is not None and card.triage.state == state:
            return card, False
        card.triage = InsightTriage(state=state, by=user_id, at=now)
    doc.version += 1
    return card, True


def mark_queued(doc: AppInsights, run_id: str, now: str) -> bool:
    """The analyze handler's stamp; False when the worker already stamped this run."""
    if doc.lastRun.runId == run_id:
        return False
    stamp_run(doc, LastRun(status=RUN_STATUS_QUEUED, trigger=TRIGGER_MANUAL, runId=run_id, queuedAt=now))
    return True


def clear_pending(doc: AppInsights, run_id: str, code: str, now: str) -> bool:
    """Fail a queued or running lastRun of this run id (a dropped job, or a crash-reclaimed message)."""
    last = doc.lastRun
    if last.runId != run_id or last.status not in _PENDING:
        return False
    stamp_run(doc, last.model_copy(update={"status": RUN_STATUS_FAILED, "error": code, "finishedAt": now}))
    return True
