"""Full coverage for models.py — schema defaults, required fields, regex enforcement.

Run: pytest tests/test_models_cov.py
"""

import os
import sys

import pytest
from pydantic import ValidationError

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from models import (  # noqa: E402
    AppSignals,
    Criticality,
    EnrichmentResult,
    EnrichmentResultLLM,
    InsightsDispatchRequest,
    InsightsDispatchResponse,
    RelatedApp,
    ResourceEfficiency,
    Risk,
    Suggestion,
    WorkloadSignal,
    _default_criticality,
    _default_efficiency,
)


def _llm_payload(**over):
    base = {
        "summary": "does x",
        "techStack": ["Redis"],
        "role": "backend-service",
        "dependencies": ["redis"],
        "confidence": "high",
        "category": "application",
        "risks": [{"severity": "high", "message": "r"}],
        "suggestions": [{"priority": "low", "message": "s"}],
        "resourceEfficiency": {"status": "under", "note": "n"},
        "criticality": {"level": "high", "reason": "why"},
        "tags": ["stateless"],
        "relatedApps": [{"name": "redis", "reason": "REDIS_HOST"}],
    }
    base.update(over)
    return base


def test_workload_signal_defaults():
    w = WorkloadSignal(name="api", kind="Deployment")
    assert w.replicas == 0 and w.qos == "" and w.limitsSet is False


def test_appsignals_minimal_defaults():
    s = AppSignals(name="a", namespace="ns")
    assert s.images == [] and s.workloads == [] and s.hasHPA is False
    assert s.changeVelocityPerDay == 0.0 and s.managedBy == ""


def test_appsignals_full_roundtrip():
    s = AppSignals.model_validate(
        {
            "name": "a", "namespace": "ns", "images": ["nginx"], "ports": [80],
            "envVarKeys": ["REDIS_HOST"], "resourceKinds": ["Deployment"],
            "hasIngress": True, "hasPVC": True, "replicas": 3, "readyReplicas": 2,
            "healthStatus": "degraded", "workloadKinds": ["Deployment"],
            "hasService": True, "hasHPA": True, "hasNetworkPolicy": True,
            "secretRefs": ["s"], "configMapRefs": ["c"], "managedBy": "helm",
            "chart": "x-1.0.0", "changeVelocityPerDay": 1.5, "incidents": 2,
            "recoveries": 1, "workloads": [{"name": "api", "kind": "Deployment",
            "replicas": 3, "qos": "Burstable", "cpu": "10m", "memory": "5Mi", "limitsSet": True}],
        }
    )
    assert s.replicas == 3 and s.workloads[0].limitsSet is True


def test_llm_valid_full():
    llm = EnrichmentResultLLM.model_validate(_llm_payload())
    assert llm.risks[0].severity == "high"
    assert llm.resourceEfficiency.status == "under"
    assert llm.criticality.level == "high"


def test_llm_requires_efficiency_and_criticality():
    p = _llm_payload()
    del p["resourceEfficiency"]
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(p)
    p = _llm_payload()
    del p["criticality"]
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(p)


@pytest.mark.parametrize(
    "field,bad",
    [
        ("confidence", "sky-high"),
        ("category", "banana"),
    ],
)
def test_llm_top_level_regex(field, bad):
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(_llm_payload(**{field: bad}))


def test_llm_nested_regex():
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(_llm_payload(risks=[{"severity": "boom", "message": "m"}]))
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(_llm_payload(suggestions=[{"priority": "boom", "message": "m"}]))
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(_llm_payload(resourceEfficiency={"status": "boom", "note": ""}))
    with pytest.raises(ValidationError):
        EnrichmentResultLLM.model_validate(_llm_payload(criticality={"level": "boom", "reason": ""}))


def test_enrichment_result_defaults():
    from datetime import datetime

    r = EnrichmentResult(summary="s", role="r", confidence="low", enrichedAt=datetime.utcnow())
    assert r.category == "application"
    assert r.risks == [] and r.suggestions == [] and r.tags == []
    assert r.resourceEfficiency.status == "unknown"
    assert r.criticality.level == "low"


def test_enrichment_result_requires_enrichedat():
    with pytest.raises(ValidationError):
        EnrichmentResult(summary="s", role="r", confidence="low")


def test_default_helpers():
    assert _default_efficiency() == ResourceEfficiency(status="unknown", note="")
    assert _default_criticality() == Criticality(level="low", reason="")


def test_dispatch_models():
    req = InsightsDispatchRequest(items=[AppSignals(name="a", namespace="n")])
    assert len(req.items) == 1
    resp = InsightsDispatchResponse(ready=["a"], pending=["b"])
    assert resp.ready == ["a"] and resp.pending == ["b"]
    assert InsightsDispatchRequest().items == []


def test_leaf_models():
    assert RelatedApp(name="redis", reason="x").name == "redis"
    assert Risk(severity="low", message="m").message == "m"
    assert Suggestion(priority="high", message="m").priority == "high"
