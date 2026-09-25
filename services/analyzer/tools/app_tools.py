"""Tools answered from the Application already fetched from the exporter: 0 Kubernetes calls."""

from __future__ import annotations

from constants import (
    CHANGE_VALUE_MAX,
    CHANGES_PER_ENTRY_MAX,
    REF_GENERATION,
    REF_SNAPSHOT,
    REF_WORKLOAD,
    SNAPSHOT_IDS_MAX,
)
from models import HistoryArgs, Run

KEY_WORKLOADS = "workloads"
KEY_CHANGES = "changes"


def _history(run: Run) -> dict:
    return run.app.get("history") or {}


def _newest_first(run: Run) -> list[dict]:
    return sorted(_history(run).get("changeLog") or [], key=lambda e: e.get("generation", 0), reverse=True)


def app_namespaces(run: Run) -> list[str]:
    """The app's namespaces minus the excluded ones: the only namespaces a tool may read."""
    items = (run.app.get("namespaces") or {}).get("items") or []
    return [i.get("name") for i in items if i.get("name") not in run.excluded]


def _cut(value: str | None) -> str | None:
    return value[:CHANGE_VALUE_MAX] if value else value


def get_app_overview(run: Run) -> tuple[dict, str]:
    app = run.app
    health = app.get("health") or {}
    history = _history(run)
    newest = next(iter(_newest_first(run)), None)
    last_change = None
    if newest:
        last_change = {k: newest.get(k) for k in ("generation", "changeClass", "severity", "isIncident", "isRecovery")}
    payload = {
        "health": {
            "status": health.get("status"),
            "reason": health.get("reason"),
            "ready": health.get("readyReplicas"),
            "total": health.get("totalReplicas"),
        },
        "namespaces": app_namespaces(run),
        "managedBy": (app.get("managed") or {}).get("by"),
        KEY_WORKLOADS: [{"kind": kind, "name": name, "namespace": ns} for kind, name, ns in run.workloads.values()],
        "generation": history.get("generation"),
        "hasDrift": history.get("hasDrift"),
        "lastChange": last_change,
        "snapshotIds": [s.get("id") for s in (app.get("snapshots") or [])[-SNAPSHOT_IDS_MAX:]],
    }
    return payload, KEY_WORKLOADS


def overview_refs(payload: dict) -> list[str]:
    return [REF_WORKLOAD.format(**w) for w in payload[KEY_WORKLOADS]] + [
        REF_SNAPSHOT.format(id=i) for i in payload["snapshotIds"]
    ]


def get_change_history(run: Run, args: HistoryArgs) -> tuple[dict, str]:
    entries = [e for e in _newest_first(run) if e.get("isIncident") or not args.onlyIncidents][: args.limit]
    changes = [
        {
            **{k: e.get(k) for k in ("generation", "detectedAt", "changeClass", "severity", "isIncident", "isRecovery")},
            KEY_CHANGES: [
                {
                    "field": c.get("field"),
                    "changeType": c.get("changeType"),
                    "oldValue": _cut(c.get("oldValue")),
                    "newValue": _cut(c.get("newValue")),
                }
                for c in (e.get("changes") or [])[:CHANGES_PER_ENTRY_MAX]
            ],
        }
        for e in entries
    ]
    return {KEY_CHANGES: changes}, KEY_CHANGES


def history_refs(payload: dict) -> list[str]:
    return [REF_GENERATION.format(generation=c["generation"]) for c in payload[KEY_CHANGES]]
