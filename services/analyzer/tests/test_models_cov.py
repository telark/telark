"""Coverage for models.py — JSON names equal the Go json tags, omitempty rules.

Run: pytest tests/test_models_cov.py
"""

import json
import os
import sys

import pytest
from pydantic import ValidationError

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from constants import DEFAULT_ANALYZER_MODEL  # noqa: E402
from models import (  # noqa: E402
    AnalyzeResponse,
    AnalyzerConfig,
    AppInsights,
    EvidenceRef,
    Insight,
    InsightTriage,
    Job,
    LastRun,
    PullProgress,
    RuntimeStatus,
    TriageRequest,
    ValidateModelRequest,
    ValidateModelResponse,
    to_json,
)

# internal/data/resources/application/insights.go:42-81
GO_INSIGHT_TAGS = [
    "id", "kind", "subject", "title", "summary", "confidence", "severity", "status",
    "evidence", "firstSeenAt", "lastSeenAt", "resolvedAt", "runs",
]
GO_LASTRUN_TAGS = [
    "status", "trigger", "runId", "queuedAt", "startedAt", "finishedAt", "error",
    "model", "steps", "toolCalls", "truncated",
]


def test_app_insights_keys_equal_go_tags():
    doc = AppInsights(
        insights=[Insight(evidence=[EvidenceRef(type="change", ref="gen:3")], resolvedAt="t")],
        lastRun=LastRun(queuedAt="t"),
    )
    out = json.loads(to_json(doc))
    assert list(out) == ["insights", "lastRun", "version"]
    assert list(out["insights"][0]) == GO_INSIGHT_TAGS
    assert out["insights"][0]["evidence"] == [{"type": "change", "ref": "gen:3"}]
    assert list(out["lastRun"]) == GO_LASTRUN_TAGS


def test_empty_optional_fields_are_omitted():
    out = json.loads(to_json(AppInsights(insights=[Insight()])))
    assert "resolvedAt" not in out["insights"][0]
    assert "queuedAt" not in out["lastRun"]
    assert out["lastRun"]["runId"] == ""
    assert out == json.loads(AppInsights(insights=[Insight()]).model_dump_json())


def test_runtime_status_pull_omitted_when_none():
    assert "pull" not in json.loads(to_json(RuntimeStatus(state="ready")))
    out = json.loads(to_json(RuntimeStatus(state="pulling", pull=PullProgress(model="m", completed=1, total=2))))
    assert out["pull"] == {"model": "m", "status": "", "completed": 1, "total": 2}


def test_job_parses_stream_fields():
    job = Job.model_validate({"namespace": "shop", "name": "api", "trigger": "incident", "generation": "7"})
    assert job.generation == 7
    with pytest.raises(ValidationError):
        Job.model_validate({"namespace": "shop", "name": "api", "trigger": "periodic", "generation": "1"})


def test_unknown_fields_ignored_and_defaults():
    cfg = AnalyzerConfig.model_validate({"enabled": True, "provider": "old"})
    assert cfg.enabled is True and cfg.model == DEFAULT_ANALYZER_MODEL
    assert cfg.autoAnalyze is False and cfg.excludedNamespaces == []
    assert json.loads(to_json(AnalyzeResponse(runId="1-0", status="queued"))) == {"runId": "1-0", "status": "queued"}
    assert ValidateModelRequest(model="qwen3:4b").model == "qwen3:4b"
    assert list(json.loads(to_json(ValidateModelResponse()))) == [
        "ok", "model", "license", "warning", "reason", "capabilities",
    ]


def test_models_omit_empty_new_fields():
    # A v2 document (no category/reason/params/triage/lastReviewAt) serializes byte-identical.
    v2 = ('{"insights":[{"id":"a","kind":"oom","subject":"deployment/api","title":"t","summary":"s",'
          '"confidence":"high","severity":"critical","status":"open","evidence":[],"firstSeenAt":"t1",'
          '"lastSeenAt":"t1","runs":1}],"lastRun":{"status":"done","trigger":"manual","runId":"1-1",'
          '"startedAt":"","finishedAt":"","error":"","model":"","steps":0,"toolCalls":0,"truncated":false},'
          '"version":2}')
    assert to_json(AppInsights.model_validate_json(v2)) == v2
    # The new fields, in the Go tag order and names.
    card = Insight(id="b", category="recommendation", reason="reliability.no_pdb", params={"workload": "web"},
                   triage=InsightTriage(state="dismissed", by="u1", at="t2"))
    out = json.loads(to_json(AppInsights(insights=[card], lastReviewAt="t3", version=1)))
    assert list(out) == ["insights", "lastRun", "version", "lastReviewAt"]
    assert list(out["insights"][0])[-4:] == ["category", "reason", "params", "triage"]
    assert out["insights"][0]["triage"] == {"state": "dismissed", "by": "u1", "at": "t2"}
    assert TriageRequest(action="dismiss").action == "dismiss"
    with pytest.raises(ValidationError):
        TriageRequest(action="snooze")
