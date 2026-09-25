"""Prompt text and schemas: the deep loop's EMIT and the fast path's narration."""

import json

from config import ANALYZER_MAX_TOOL_CALLS
from constants import (
    CONFIDENCE_HIGH,
    CONFIDENCE_LOW,
    CONFIDENCE_MEDIUM,
    EVIDENCE_TYPE_CHANGE,
    EVIDENCE_TYPE_EVENT,
    EVIDENCE_TYPE_SNAPSHOT,
    EVIDENCE_TYPE_WORKLOAD,
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
    INSIGHT_SEVERITY_INFO,
    INSIGHT_SEVERITY_WARNING,
    JSON_COMPACT_SEPARATORS,
    MAX_EVIDENCE_PER_INSIGHT,
    MAX_INSIGHT_SUMMARY_LENGTH,
    MAX_INSIGHT_TITLE_LENGTH,
    MAX_INSIGHTS_PER_RUN,
    MAX_TOOL_CALLS_PER_STEP,
    NARRATE_SUMMARY_MAX,
    NARRATE_TITLE_MAX,
    SUBJECT_SEPARATOR,
)
import messages
from models import Candidate, Job

SYSTEM_PROMPT = f"""You are a read-only investigator for one Kubernetes application. You cannot change anything.
Find what is wrong with the application and why, using only the tools.

Rules:
- Start with get_app_overview. Use at most {ANALYZER_MAX_TOOL_CALLS} tool calls in total and at most \
{MAX_TOOL_CALLS_PER_STEP} per turn.
- Only ask about workloads listed by get_app_overview, with the kind in lower case.
- Stop as soon as the evidence suffices: answer with one short sentence and no tool call.
- Never guess. Every finding must rest on tool results from this conversation.

Evidence refs: cite only refs built from tool results of this conversation, copied exactly.
- workload: workload:<kind>/<name>, for example workload:deployment/api
- change: gen:<generation> from get_change_history
- snapshot: snap:<id> from the overview snapshotIds
- event: <reason>@<object>@<last> from get_recent_events, for example BackOff@Pod/api-7c9d8-x2x4k@2026-01-01T10:00:00Z

Output, when asked to emit: at most {MAX_INSIGHTS_PER_RUN} insights, one per workload. subject is <kind>/<name> of a \
listed workload; title at most {MAX_INSIGHT_TITLE_LENGTH} characters; summary at most {MAX_INSIGHT_SUMMARY_LENGTH} \
characters, stating the cause and what shows it; at most {MAX_EVIDENCE_PER_INSIGHT} evidence refs. Emit an empty \
list when nothing is wrong."""

EMIT_MESSAGE = (
    "Emit now: return the insights JSON for what the tool results show. "
    "Cite only refs from this conversation, and return an empty list when nothing is wrong."
)

EMIT_RETRY_MESSAGE = "That output did not match the schema ({detail}). Emit the insights JSON again, exactly per the schema."

_EVIDENCE = {
    "type": "object",
    "required": ["type", "ref"],
    "properties": {
        "type": {"enum": [EVIDENCE_TYPE_CHANGE, EVIDENCE_TYPE_SNAPSHOT, EVIDENCE_TYPE_EVENT, EVIDENCE_TYPE_WORKLOAD]},
        "ref": {"type": "string"},
    },
}

_INSIGHT = {
    "type": "object",
    "required": ["kind", "subject", "title", "summary", "confidence", "severity", "evidence"],
    "additionalProperties": False,
    "properties": {
        "kind": {"enum": [
            INSIGHT_KIND_CRASHLOOP,
            INSIGHT_KIND_OOM,
            INSIGHT_KIND_IMAGE_PULL,
            INSIGHT_KIND_PROBE_FAILURE,
            INSIGHT_KIND_SCHEDULING,
            INSIGHT_KIND_ROLLOUT_STUCK,
            INSIGHT_KIND_CONFIG_CHANGE_REGRESSION,
            INSIGHT_KIND_RESOURCE_PRESSURE,
            INSIGHT_KIND_OTHER,
        ]},
        "subject": {"type": "string"},
        "title": {"type": "string", "maxLength": MAX_INSIGHT_TITLE_LENGTH},
        "summary": {"type": "string", "maxLength": MAX_INSIGHT_SUMMARY_LENGTH},
        "confidence": {"enum": [CONFIDENCE_LOW, CONFIDENCE_MEDIUM, CONFIDENCE_HIGH]},
        "severity": {"enum": [INSIGHT_SEVERITY_INFO, INSIGHT_SEVERITY_WARNING, INSIGHT_SEVERITY_CRITICAL]},
        "evidence": {"type": "array", "maxItems": MAX_EVIDENCE_PER_INSIGHT, "items": _EVIDENCE},
    },
}

EMIT_SCHEMA = {
    "type": "object",
    "required": ["insights"],
    "additionalProperties": False,
    "properties": {"insights": {"type": "array", "maxItems": MAX_INSIGHTS_PER_RUN, "items": _INSIGHT}},
}


def user_message(job: Job, app: dict) -> str:
    """The run's facts; the newest change-log entry is the change the trigger refers to."""
    log = (app.get("history") or {}).get("changeLog") or []
    newest = max(log, key=lambda e: e.get("generation", 0), default=None)
    last_change = {k: newest.get(k) for k in ("generation", "changeClass", "severity")} if newest else None
    return json.dumps(
        {"namespace": job.namespace, "name": job.name, "trigger": job.trigger, "lastChange": last_change},
        separators=JSON_COMPACT_SEPARATORS,
    )


# Byte-stable and sent first: Ollama serves the identical prefix from its cache.
# A draft to polish, not facts to write from: granite4:350m invents names and drops the cause from bare
# facts, and copies any worked example into every answer (measured on telark-dev, 2026-09-24).
NARRATE_SYSTEM = (
    "You polish Kubernetes findings for an operator, one item per finding, in the given order. Each finding has a "
    "draft title and summary written from its facts. Rewrite them as clear plain sentences: the title (at most "
    f"{NARRATE_TITLE_MAX} characters) starts with the finding's name exactly as given; the summary (at most "
    f"{NARRATE_SUMMARY_MAX} characters) keeps every name, number, image and reason from the draft. Keep the phrase "
    "given as what and every number exactly. Add nothing that is not in the facts: no advice, no guesses, no "
    "markdown."
)


def narrate_message(candidates: list[Candidate]) -> str:
    return json.dumps(
        [{"i": i, "kind": c.kind, "name": c.subject.partition(SUBJECT_SEPARATOR)[2], "title": c.title,
          "summary": c.summary, "what": messages.what(c.reason, c.params), "params": c.params}
         for i, c in enumerate(candidates)],
        separators=JSON_COMPACT_SEPARATORS,
    )


def narrate_schema(n: int) -> dict:
    item = {
        "type": "object",
        "required": ["title", "summary"],
        "additionalProperties": False,
        "properties": {
            "title": {"type": "string", "maxLength": NARRATE_TITLE_MAX},
            "summary": {"type": "string", "maxLength": NARRATE_SUMMARY_MAX},
        },
    }
    return {
        "type": "object",
        "required": ["insights"],
        "additionalProperties": False,
        "properties": {"insights": {"type": "array", "minItems": n, "maxItems": n, "items": item}},
    }
