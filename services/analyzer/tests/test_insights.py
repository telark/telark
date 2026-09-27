"""The code-owned insight lifecycle: ids, validation, merge, resolve, prune, run stamps.

Pure functions only (no Redis): the model proposes, this code decides what is
stored, so a wrong transition here strands or resurrects insights.

Run: python -m pytest tests/test_insights.py
"""

import hashlib
import os
import sys

import pytest
from pydantic import ValidationError

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import insights  # noqa: E402
import models  # noqa: E402
from models import AppInsights, EvidenceRef, Insight, LastRun, Run  # noqa: E402

NOW = "2026-09-23T12:00:00Z"
EARLIER = "2026-09-23T10:00:00Z"
REFS = ["workload:deployment/api", "gen:7", "BackOff@Pod/api-7d9fb-x2k4q@2026-09-23T11:00:00Z"]


def _run():
    app = {"resources": [
        {"namespace": "shop", "kind": "Deployment", "name": "api"},
        {"namespace": "shop", "kind": "StatefulSet", "name": "db"},
        {"namespace": "shop", "kind": "Service", "name": "api"},
    ]}
    run = Run("shop", "api", app, [])
    run.refs.update(REFS)
    return run


def _emitted(subject="deployment/api", kind="crashloop", severity="warning", refs=REFS[:2], **kw):
    fields = {"title": "Pods crash", "summary": "api restarts", "confidence": "high"} | kw
    return models.Emitted(kind=kind, subject=subject, severity=severity,
                          evidence=[EvidenceRef(type="workload", ref=r) for r in refs], **fields)


def _insight(subject="deployment/api", status="open", last_seen=EARLIER, kind="crashloop", **kw):
    return Insight(id=insights.insight_id("shop", "api", subject, "shop"), kind=kind, subject=subject,
                   status=status, firstSeenAt=EARLIER, lastSeenAt=last_seen, runs=1, **kw)


def _ready(desired=2, ready=2, waiting=None):
    return {"desired": desired, "ready": ready, "pods": [{"name": "api-7d9fb-x2k4q", "waitingReason": waiting}]}


def test_insight_id_stable():
    first = insights.insight_id("shop", "api", "deployment/api", "shop")
    assert first == insights.insight_id("shop", "api", "deployment/api", "shop")
    assert len(first) == 16
    assert first != insights.insight_id("shop", "api", "statefulset/db", "shop")
    assert first != insights.insight_id("other", "api", "deployment/api", "shop")
    assert first != insights.insight_id("shop", "api", "deployment/api", "shop-prod"), "per workload namespace"

    # kind is an attribute, never keyed: a relabelled failure updates the same card.
    doc = AppInsights()
    insights.merge(doc, [_emitted(kind="crashloop")], EARLIER, False, "shop", "api")
    insights.merge(doc, [_emitted(kind="oom")], NOW, False, "shop", "api")
    assert [(i.id, i.kind, i.status, i.runs) for i in doc.insights] == [(first, "oom", "updated", 2)]


def test_emitted_is_strict():
    with pytest.raises(ValidationError):
        _emitted(status="resolved")  # extra field
    with pytest.raises(ValidationError):
        _emitted(kind="meltdown")
    with pytest.raises(ValidationError):
        _emitted(refs=["gen:1"] * 5)  # max 4 refs


def test_validate_canonical_subject_and_refs():
    run = _run()
    valid = insights.validate([
        _emitted(subject="Deployment/api", refs=[REFS[0], "gen:999", REFS[1], REFS[0]], title="t" * 200, summary="s" * 900),
        _emitted(subject="deployment/ghost"),
        _emitted(subject="noslash"),
    ], run)
    assert len(valid) == 1
    e = valid[0]
    assert e.subject == "deployment/api"
    assert [r.ref for r in e.evidence] == REFS[:2]  # unknown dropped, duplicates collapsed
    assert (len(e.title), len(e.summary)) == (120, 400)


def test_validate_zero_refs_dropped_unless_other():
    run = _run()
    valid = insights.validate([
        _emitted(refs=["gen:999"]),
        _emitted(subject="statefulset/db", kind="other", refs=[], confidence="high"),
    ], run)
    assert [(e.subject, e.kind, e.confidence) for e in valid] == [("statefulset/db", "other", "low")]


def test_validate_same_subject_keeps_higher_severity():
    run = _run()
    valid = insights.validate([
        _emitted(severity="warning", title="first"),
        _emitted(severity="critical", title="second"),
        _emitted(severity="info", title="third"),
    ], run)
    assert [(e.severity, e.title) for e in valid] == [("critical", "second")]


def test_validate_bounds_params():
    run = _run()
    e = _emitted(params={"workload": "x" * 500, "injected": "y"})
    (valid,) = insights.validate([e], run)
    assert set(valid.params) == {"workload", "namespace"}
    assert len(valid.params["workload"]) == 120


def test_validate_stamps_the_workload_namespace():
    app = {"resources": [{"namespace": "shop-db", "kind": "StatefulSet", "name": "db"},
                         {"namespace": "shop", "kind": "Deployment", "name": "web"},
                         {"namespace": "shop-prod", "kind": "Deployment", "name": "web"}]}
    run = Run("shop", "api", app, [])
    run.refs.update(["workload:statefulset/db", "workload:deployment/web"])
    deep = _emitted(subject="statefulset/db", refs=["workload:statefulset/db"])
    fast = _emitted(subject="deployment/web", refs=["workload:deployment/web"],
                    params={"workload": "web", "namespace": "shop-prod"})
    got = {e.subject: e.params for e in insights.validate([deep, fast], run)}
    # Deep mode's model names no namespace: the card still carries its workload's, like fast mode's.
    assert got == {"statefulset/db": {"namespace": "shop-db"},
                   "deployment/web": {"workload": "web", "namespace": "shop-prod"}}
    assert deep.params == {}, "the model's insight is never mutated"
    ghost = _emitted(subject="deployment/web", params={"namespace": "elsewhere"})
    assert insights.validate([ghost], run) == [], "a namespace the app does not run the workload in"
    doc = AppInsights()
    insights.merge(doc, insights.validate([deep], run), NOW, False, "shop", "api")
    assert doc.insights[0].params == {"namespace": "shop-db"}
    assert doc.insights[0].id == insights.insight_id("shop", "api", "statefulset/db", "shop-db")


def _legacy_id(subject):
    return hashlib.sha256(f"shop|api|{subject}".encode()).hexdigest()[:16]


def test_legacy_ids_move_to_the_new_id_once():
    run = _run()
    resolved = _insight(status="resolved", resolvedAt=EARLIER).model_copy(update={"id": _legacy_id("deployment/api")})
    doc = AppInsights(insights=[resolved])
    stats = insights.merge(doc, insights.validate([_emitted()], run), NOW, False, "shop", "api")
    (card,) = doc.insights
    new_id = insights.insight_id("shop", "api", "deployment/api", "shop")
    assert (card.id, card.status, card.runs, card.firstSeenAt) == (new_id, "open", 2, EARLIER)
    assert stats.reopened == [new_id] and stats.touched == {new_id}
    # A legacy card that names another namespace belongs to that workload, not to this one.
    other = _insight(params={"namespace": "shop-prod"}).model_copy(update={"id": _legacy_id("deployment/api")})
    doc = AppInsights(insights=[other.model_copy()])
    insights.merge(doc, insights.validate([_emitted()], run), NOW, False, "shop", "api")
    assert [c.id for c in doc.insights] == [other.id, new_id]


def test_merge_new_update_and_clamp():
    doc = AppInsights()
    stats = insights.merge(doc, [_emitted(refs=REFS[:1])], EARLIER, False, "shop", "api")
    [card] = doc.insights
    assert (card.status, card.runs, card.firstSeenAt, card.lastSeenAt, card.confidence) == (
        "open", 1, EARLIER, EARLIER, "low")  # one ref -> clamped low
    assert (stats.created, stats.updated, stats.reopened, stats.touched) == ([card.id], [], [], {card.id})

    stats = insights.merge(doc, [_emitted(title="new title", severity="critical")], NOW, False, "shop", "api")
    assert (card.status, card.runs, card.firstSeenAt, card.lastSeenAt) == ("updated", 2, EARLIER, NOW)
    assert (card.title, card.severity, card.confidence, len(card.evidence)) == ("new title", "critical", "high", 2)
    assert stats.updated == [card.id]

    insights.merge(doc, [_emitted()], NOW, True, "shop", "api")
    assert card.confidence == "low"  # truncated run


def test_merge_reopen():
    resolved = _insight(status="resolved", resolvedAt=EARLIER)
    doc = AppInsights(insights=[resolved])
    stats = insights.merge(doc, [_emitted()], NOW, False, "shop", "api")
    assert (resolved.status, resolved.resolvedAt, resolved.firstSeenAt, resolved.lastSeenAt, resolved.runs) == (
        "open", "", EARLIER, NOW, 2)
    assert stats.reopened == [resolved.id] and stats.touched == {resolved.id}
    assert "resolvedAt" not in models.to_json(doc)


def test_omission_never_resolves():
    kept = _insight(subject="statefulset/db")
    doc = AppInsights(insights=[kept.model_copy()])
    stats = insights.merge(doc, [_emitted()], NOW, False, "shop", "api")
    assert doc.insights[0] == kept
    assert kept.id not in stats.touched


def test_resolve_on_recovery():
    doc = AppInsights(insights=[_insight(), _insight(subject="statefulset/db", status="updated"),
                                _insight(subject="daemonset/x", status="resolved", resolvedAt=EARLIER)])
    resolved = insights.resolve_all(doc, NOW)
    assert resolved == [doc.insights[0].id, doc.insights[1].id]
    assert [(i.status, i.resolvedAt) for i in doc.insights] == [
        ("resolved", NOW), ("resolved", NOW), ("resolved", EARLIER)]


def test_resolve_observed_ready():
    run = _run()
    run.status_cache[("shop", "deployment/api")] = _ready()
    run.status_cache[("shop", "statefulset/db")] = _ready(ready=1)
    doc = AppInsights(insights=[_insight(), _insight(subject="statefulset/db")])
    resolved = insights.resolve_observed(doc, run, NOW, set())
    assert resolved == [doc.insights[0].id]
    assert [(i.status, i.resolvedAt) for i in doc.insights] == [("resolved", NOW), ("open", "")]

    # a waiting pod or a subject never fetched this run keeps the card open
    run.status_cache[("shop", "deployment/api")] = _ready(waiting="CrashLoopBackOff")
    doc = AppInsights(insights=[_insight(), _insight(subject="daemonset/x")])
    assert insights.resolve_observed(doc, run, NOW, set()) == []


def test_paused_workload_resolves_its_rollout_cards():
    # Live (e2e-i-ops): the rule stopped firing on a paused Deployment, but its old card stayed open 0/2 forever.
    run = _run()
    run.status_cache[("shop", "deployment/api")] = {**_ready(ready=0), "paused": True}
    for kind in ("rollout_stuck", "config_change_regression", "other"):
        doc = AppInsights(insights=[_insight(kind=kind)])
        assert insights.resolve_observed(doc, run, NOW, set()) == [doc.insights[0].id], kind
    # Pausing does not stop a pod symptom: those kinds still need every replica ready.
    for kind in ("crashloop", "oom", "image_pull", "scheduling", "probe_failure", "resource_pressure"):
        assert insights.resolve_observed(AppInsights(insights=[_insight(kind=kind)]), run, NOW, set()) == [], kind
    # A waiting pod or a newer warning keeps even a rollout card open.
    run.status_cache[("shop", "deployment/api")] = {**_ready(ready=0, waiting="CrashLoopBackOff"), "paused": True}
    assert insights.resolve_observed(AppInsights(insights=[_insight(kind="rollout_stuck")]), run, NOW, set()) == []
    run.status_cache[("shop", "deployment/api")] = {**_ready(ready=0), "paused": True}
    run.events_cache.append({"reason": "BackOff", "object": "Pod/api-7d9fb-x2k4q", "last": NOW, "namespace": "shop"})
    assert insights.resolve_observed(AppInsights(insights=[_insight(kind="rollout_stuck")]), run, NOW, set()) == []


def test_gone_workload_resolves_and_emits_nothing():
    app = {"resources": [{"namespace": "shop", "kind": "Deployment", "name": "api"},
                         {"namespace": "shop-b", "kind": "Deployment", "name": "api"}]}
    run = Run("shop", "api", app, [])
    run.refs.update(REFS)
    run.gone.add(("shop-b", "deployment/api"))
    gone = _insight(params={"namespace": "shop-b"})
    gone.id = insights.insight_id("shop", "api", "deployment/api", "shop-b")
    doc = AppInsights(insights=[_insight(params={"namespace": "shop"}), gone])
    # Only the deleted copy resolves: the other namespace's workload was never read this run.
    assert insights.resolve_observed(doc, run, NOW, set()) == [gone.id]
    assert [c.status for c in doc.insights] == ["open", "resolved"]
    fresh = [_emitted(params={"namespace": ns}) for ns in ("shop", "shop-b")]
    assert [e.params["namespace"] for e in insights.validate(fresh, run)] == ["shop"]


def test_observed_resolves_a_card_whose_namespace_was_excluded():
    app = {"resources": [{"namespace": "shop", "kind": "Deployment", "name": "api"},
                         {"namespace": "shop-db", "kind": "StatefulSet", "name": "db"}]}
    run = Run("shop", "api", app, ["shop-db"])
    hidden = _insight(subject="statefulset/db", params={"namespace": "shop-db"})
    unread = _insight(params={"namespace": "shop"})
    doc = AppInsights(insights=[hidden, unread])
    # No read ever reaches an excluded namespace again: the card resolves (and is pruned 7 days later).
    assert insights.resolve_observed(doc, run, NOW, set()) == [hidden.id]
    assert [c.status for c in doc.insights] == ["resolved", "open"]


def test_newer_warning_blocks_resolve():
    run = _run()
    run.status_cache[("shop", "deployment/api")] = _ready()
    run.events_cache.append({"reason": "BackOff", "object": "Pod/api-7d9fb-x2k4q", "last": "2026-09-23T11:00:00Z",
                            "namespace": "shop"})
    run.events_cache.append({"reason": "Unhealthy", "object": "Pod/db-0", "last": NOW, "namespace": "shop"})
    assert insights.resolve_observed(AppInsights(insights=[_insight()]), run, NOW, set()) == []
    # the same warning older than lastSeenAt does not block
    doc = AppInsights(insights=[_insight(last_seen="2026-09-23T11:30:00Z")])
    assert insights.resolve_observed(doc, run, NOW, set()) == [doc.insights[0].id]
    # the newer warning belongs to a pod the fix replaced (live: rl-oomhist): it no longer blocks
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": "api-6c5d4-new01"}}]
    assert insights.resolve_observed(AppInsights(insights=[_insight()]), run, NOW, set()) != []
    run.pods_cache[("shop", "deployment/api")] = [{"metadata": {"name": "api-7d9fb-x2k4q"}}]
    assert insights.resolve_observed(AppInsights(insights=[_insight()]), run, NOW, set()) == []


def test_resolve_skips_emitted_this_run():
    run = _run()
    run.status_cache[("shop", "deployment/api")] = _ready()
    doc = AppInsights()
    stats = insights.merge(doc, [_emitted()], NOW, False, "shop", "api")
    assert insights.resolve_observed(doc, run, NOW, stats.touched) == []
    assert doc.insights[0].status == "open"


def test_prune_resolved():
    doc = AppInsights(insights=[
        _insight(subject="deployment/api", status="resolved", resolvedAt="2026-09-16T11:59:59Z"),  # 7 d + 1 s
        _insight(subject="statefulset/db", status="resolved", resolvedAt="2026-09-16T12:00:00Z"),  # exactly 7 d
        _insight(subject="daemonset/x", last_seen="2026-09-01T00:00:00Z"),  # old but open
    ])
    insights.prune(doc, NOW)
    assert [i.subject for i in doc.insights] == ["statefulset/db", "daemonset/x"]


def test_stamp_run_bumps_version():
    doc = AppInsights(version=4)
    insights.stamp_run(doc, LastRun(status="running", runId="1-1"))
    insights.stamp_run(doc, LastRun(status="done", runId="1-1"))
    assert (doc.version, doc.lastRun.status) == (6, "done")


def test_mark_queued_skips_stamped_run():
    doc = AppInsights()
    assert insights.mark_queued(doc, "5-1", NOW) is True
    assert (doc.lastRun.status, doc.lastRun.trigger, doc.lastRun.runId, doc.lastRun.queuedAt, doc.version) == (
        "queued", "manual", "5-1", NOW, 1)

    # the worker already stamped this run (running): the handler must not roll it back
    insights.stamp_run(doc, LastRun(status="running", trigger="manual", runId="5-1", queuedAt=NOW))
    assert insights.mark_queued(doc, "5-1", NOW) is False
    assert (doc.lastRun.status, doc.version) == ("running", 2)


def test_clear_pending_only_matching_run():
    other = LastRun(status="queued", trigger="manual", runId="9-9", queuedAt=EARLIER)
    doc = AppInsights(lastRun=other, version=3)
    assert insights.clear_pending(doc, "5-1", "job_expired", NOW) is False
    assert (doc.lastRun, doc.version) == (other, 3)

    for status, started in (("queued", ""), ("running", EARLIER)):
        doc = AppInsights(lastRun=LastRun(status=status, trigger="manual", runId="5-1", queuedAt=EARLIER,
                                          startedAt=started), version=3)
        assert insights.clear_pending(doc, "5-1", "job_expired", NOW) is True
        run = doc.lastRun
        assert (run.status, run.error, run.runId, run.queuedAt, run.startedAt, run.finishedAt, doc.version) == (
            "failed", "job_expired", "5-1", EARLIER, started, NOW, 4)

    # a finished run with the same id is never rewritten
    done = LastRun(status="done", trigger="manual", runId="5-1")
    doc = AppInsights(lastRun=done, version=3)
    assert insights.clear_pending(doc, "5-1", "job_expired", NOW) is False
    assert (doc.lastRun, doc.version) == (done, 3)


def test_narrate_sets_only_title_and_summary():
    card = _insight(title="template", summary="template summary", severity="critical", confidence="high",
                    evidence=[EvidenceRef(type="workload", ref=REFS[0])])
    other = _insight(subject="statefulset/db", title="db", summary="db summary")
    doc = AppInsights(insights=[card.model_copy(), other.model_copy()], version=4)

    ids = [card.id, "unknown-id", other.id]
    texts = [("  api crashes ", " The api pod restarts. "), ("x", "y"), ("", "only a summary")]
    assert insights.narrate(doc, ids, texts) == [card.id]

    got, kept = doc.insights
    assert (got.title, got.summary) == ("api crashes", "The api pod restarts.")
    # Code-owned fields never move.
    assert got.model_dump(exclude={"title", "summary"}) == card.model_dump(exclude={"title", "summary"})
    assert kept == other and doc.version == 4
    # Longer than the document caps: cut, like validate() does.
    insights.narrate(doc, [card.id], [("t" * 200, "s" * 500)])
    assert (len(doc.insights[0].title), len(doc.insights[0].summary)) == (120, 400)


def test_merge_sets_incident_category_reason_params():
    doc = AppInsights()
    first = _emitted(reason="crashloop.exit_1", params={"workload": "api", "exitCode": "1"})
    stats = insights.merge(doc, [first], EARLIER, False, "shop", "api")
    (card,) = doc.insights
    assert (card.category, card.reason, card.params) == ("incident", "crashloop.exit_1",
                                                         {"workload": "api", "exitCode": "1"})
    assert stats.created == [card.id]
    # The next run's sub-reason and params replace the old ones on the same card.
    insights.merge(doc, [_emitted(kind="oom", reason="oom.limit", params={"limit": "64Mi"})], NOW, False, "shop", "api")
    assert (card.kind, card.reason, card.params, card.status) == ("oom", "oom.limit", {"limit": "64Mi"}, "updated")
    # A deep-mode insight carries no reason: the card keeps the category, and the UI falls back on the kind.
    insights.merge(doc, [_emitted()], NOW, False, "shop", "api")
    assert (card.category, card.reason, card.params) == ("incident", "", {})
    dumped = card.model_dump()
    assert "reason" not in dumped and "params" not in dumped and dumped["category"] == "incident"


# ---- recommendations lifecycle (D14), triage (D15), category scoping (S10) -------------------------------------
from dataclasses import dataclass, field as dc_field  # noqa: E402

from models import InsightTriage  # noqa: E402

REC_NOW, LATER = "2026-09-24T12:00:00Z", "2026-09-24T14:00:00Z"


@dataclass
class _Finding:
    reason: str = "reliability.no_pdb"
    kind: str = "reliability"
    subject: str = "deployment/api"
    key: str = "deployment/api"
    namespace: str = "shop"
    severity: str = "info"
    confidence: str = "high"
    evidence: list = dc_field(default_factory=lambda: [EvidenceRef(type="spec", ref="spec:deployment/api#replicas")])
    params: dict = dc_field(default_factory=lambda: {"workload": "api", "namespace": "shop", "replicas": "2"})


def _evaluated(*findings):
    return {(f.reason, f.namespace, f.key) for f in findings}


def _merge(doc, findings, evaluated=None, now=REC_NOW):
    return insights.merge_recommendations(doc, findings, _evaluated(*findings) if evaluated is None else evaluated,
                                          now, "shop", "api")


def test_resolve_all_and_observed_skip_recommendations():
    doc = AppInsights(insights=[_insight(), _insight(subject="service/api", category="recommendation",
                                                     reason="networking.no_network_policy")])
    doc.insights[0].triage = InsightTriage(state="acknowledged", by="u", at=EARLIER)
    assert insights.resolve_all(doc, NOW) == [doc.insights[0].id]
    assert doc.insights[1].status == "open" and doc.insights[0].triage is None, "a resolve clears the triage"
    # resolve_observed would KeyError on a 'service/…' subject: recommendations are never its business.
    doc = AppInsights(insights=[_insight(subject="service/api", category="recommendation", reason="x")])
    run = _run()
    assert insights.resolve_observed(doc, run, NOW, set()) == [] and doc.insights[0].status == "open"


def test_merge_recommendations_lifecycle():
    doc = AppInsights()
    f = _Finding()
    stats = _merge(doc, [f])
    (card,) = doc.insights
    iid = insights.recommendation_id("shop", "api", "deployment/api", "reliability.no_pdb", "shop")
    assert stats.created == [iid] == [card.id]
    assert card.id != insights.insight_id("shop", "api", "deployment/api", "shop")
    assert (card.category, card.reason, card.status, card.runs, card.firstSeenAt) == (
        "recommendation", "reliability.no_pdb", "open", 1, REC_NOW)
    assert card.title == "api has no disruption budget" and "runs 2 replicas" in card.summary

    # Same facts: only lastSeenAt moves, no event. A volatile measurement does not count as a change.
    stats = _merge(doc, [_Finding(params={**f.params, "usage": "12Mi"})], now=LATER)
    assert (stats.updated, card.status, card.lastSeenAt, card.runs, card.params["usage"]) == (
        [], "open", LATER, 1, "12Mi")
    # New facts: updated.
    stats = _merge(doc, [_Finding(params={**f.params, "replicas": "3"})])
    assert stats.updated == [iid] and card.status == "updated" and card.runs == 2 and "3 replicas" in card.summary
    # Not evaluated (a failed read): untouched.
    stats = _merge(doc, [], evaluated=set())
    assert stats.resolved == [] and card.status == "updated"
    # Evaluated and not found: resolved.
    stats = _merge(doc, [], evaluated=_evaluated(f), now=LATER)
    assert stats.resolved == [iid] and (card.status, card.resolvedAt) == ("resolved", LATER)
    # Found again: reopened.
    stats = _merge(doc, [f])
    assert stats.reopened == [iid] and (card.status, card.resolvedAt, card.runs) == ("open", "", 3)
    # Incident cards are never touched by a review.
    doc.insights.append(_insight())
    _merge(doc, [], evaluated={("reliability.no_pdb", "shop", "deployment/api")})
    assert doc.insights[-1].status == "open"


def test_usage_rule_ids_include_container_and_resource():
    cpu = _Finding(reason="resources.overprovisioned", kind="resources", key="deployment/api#app/cpu",
                   params={"namespace": "shop", "container": "app", "resource": "cpu", "workload": "api",
                           "request": "150m", "usage": "10m", "suggested": "20m", "samples": "13"})
    mem = _Finding(reason="resources.overprovisioned", kind="resources", key="deployment/api#app/memory",
                   params={**cpu.params, "resource": "memory", "request": "160Mi", "usage": "20Mi",
                           "suggested": "32Mi"})
    doc = AppInsights()
    _merge(doc, [cpu, mem])
    assert len({c.id for c in doc.insights}) == 2 and {c.subject for c in doc.insights} == {"deployment/api"}
    stats = _merge(doc, [cpu], evaluated=_evaluated(cpu, mem))
    assert [c.params["resource"] for c in doc.insights if c.id in stats.resolved] == ["memory"]


def test_dismiss_kept_until_fingerprint_changes():
    doc = AppInsights()
    f = _Finding()
    _merge(doc, [f])
    (card,) = doc.insights
    insights.apply_triage(doc, card.id, "dismiss", "u1", REC_NOW)
    _merge(doc, [f], now=LATER)
    assert card.triage.state == "dismissed", "unchanged facts keep the dismissal"
    _merge(doc, [_Finding(severity="warning")])
    assert card.triage is None and card.status == "updated", "new facts clear it"
    insights.apply_triage(doc, card.id, "dismiss", "u1", REC_NOW)
    _merge(doc, [], evaluated=_evaluated(f))
    assert card.triage is None and card.status == "resolved"


def test_cap_40_most_severe_first():
    findings = [_Finding(key=f"deployment/w{i}", subject=f"deployment/w{i}",
                         severity="warning" if i < 5 else "info") for i in range(45)]
    doc = AppInsights()
    stats = _merge(doc, findings)
    assert len(stats.created) == 40 and len(doc.insights) == 40
    assert sum(c.severity == "warning" for c in doc.insights) == 5
    # A card that resolves frees its slot in the same review: the next most severe finding fills it.
    stats = _merge(doc, findings[1:], evaluated=_evaluated(*findings))
    assert len(stats.created) == 1 and sum(c.status != "resolved" for c in doc.insights) == 40
    stats = _merge(doc, findings[1:], evaluated=_evaluated(*findings))
    assert stats.created == [] and sum(c.status != "resolved" for c in doc.insights) == 40


def test_cap_admits_the_most_severe_not_the_first():
    params = {"workload": "api", "namespace": "shop", "containers": "app"}
    infos = [_Finding(reason="security.seccomp_unset", kind="security", key=f"deployment/w{i}",
                      subject=f"deployment/w{i}", params=params) for i in range(40)]
    doc = AppInsights()
    _merge(doc, infos)
    critical = _Finding(reason="security.privileged", kind="security", severity="critical", params=params)
    stats = _merge(doc, [critical] + infos, now=LATER)
    crit_id = insights.recommendation_id("shop", "api", "deployment/api", "security.privileged", "shop")
    assert stats.created == [crit_id] and stats.resolved == []
    recs = [c for c in doc.insights if c.category == "recommendation"]
    assert len(recs) == 40 and crit_id in {c.id for c in recs}
    # The info card pushed past the cap is removed, not resolved: it is still true.
    assert "deployment/w39" not in {c.subject for c in recs} and all(c.status != "resolved" for c in recs)
    # It comes back once there is room.
    stats = _merge(doc, infos, evaluated=_evaluated(critical, *infos))
    assert stats.resolved == [crit_id] and len(stats.created) == 1


def test_dismissed_cards_do_not_starve_new_ones():
    infos = [_Finding(key=f"deployment/w{i}", subject=f"deployment/w{i}") for i in range(40)]
    doc = AppInsights()
    _merge(doc, infos)
    for card in list(doc.insights):
        insights.apply_triage(doc, card.id, "dismiss", "u1", REC_NOW)
    new = _Finding(key="deployment/new", subject="deployment/new")
    stats = _merge(doc, infos + [new], now=LATER)
    assert len(stats.created) == 1 and len(doc.insights) == 41
    assert sum(c.triage is not None and c.triage.state == "dismissed" for c in doc.insights) == 40


def test_apply_triage_matrix():
    doc = AppInsights()
    _merge(doc, [_Finding()])
    rec = doc.insights[0]
    incident = _insight()
    doc.insights.append(incident)

    card, changed = insights.apply_triage(doc, incident.id, "acknowledge", "u1", REC_NOW)
    assert changed and card.triage == InsightTriage(state="acknowledged", by="u1", at=REC_NOW)
    assert insights.apply_triage(doc, incident.id, "acknowledge", "u2", LATER) == (card, False), "no-op"
    card, changed = insights.apply_triage(doc, rec.id, "acknowledge", "u1", REC_NOW)
    assert changed and card.triage.state == "acknowledged"
    card, changed = insights.apply_triage(doc, rec.id, "dismiss", "u1", REC_NOW)
    assert changed and card.triage.state == "dismissed"
    with pytest.raises(insights.TriageError) as err:
        insights.apply_triage(doc, incident.id, "dismiss", "u1", REC_NOW)
    assert err.value.code == "invalid_triage"
    card, changed = insights.apply_triage(doc, rec.id, "reopen", "u1", REC_NOW)
    assert changed and card.triage is None
    assert insights.apply_triage(doc, rec.id, "reopen", "u1", REC_NOW) == (card, False), "no-op"
    incident.status = "resolved"
    with pytest.raises(insights.TriageError) as err:
        insights.apply_triage(doc, incident.id, "acknowledge", "u1", REC_NOW)
    assert err.value.code == "invalid_triage"
    with pytest.raises(insights.TriageError) as err:
        insights.apply_triage(doc, "nope", "acknowledge", "u1", REC_NOW)
    assert err.value.code == "insight_not_found"


def test_stamp_review_bumps_version_not_last_run():
    doc = AppInsights(version=4, lastRun=LastRun(status="done", runId="1-1"))
    insights.stamp_review(doc, REC_NOW)
    assert (doc.version, doc.lastReviewAt, doc.lastRun.runId) == (5, REC_NOW, "1-1")
