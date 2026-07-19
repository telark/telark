"""Full coverage for the prompt builder — rendering, empty-signal marking, versioning.

Run: pytest tests/test_prompt_cov.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from models import AppSignals, WorkloadSignal  # noqa: E402
from prompts import k8s_app_analyzer_prompt as P  # noqa: E402


def test_fmt():
    assert P._fmt([]) == "(none provided)"
    assert P._fmt(["a", "b"]) == "a, b"
    assert P._fmt([80, 443]) == "80, 443"


def test_fmt_workloads_empty():
    assert P._fmt_workloads([]) == "  (none provided)"


def test_fmt_workloads_renders_each():
    out = P._fmt_workloads(
        [WorkloadSignal(name="api", kind="Deployment", replicas=2, qos="Burstable",
                        cpu="120m", memory="256Mi", limitsSet=False)]
    )
    assert "api Deployment x2" in out
    assert "QoS=Burstable cpu=120m mem=256Mi" in out
    assert "limitsSet=False" in out


def test_fmt_workloads_unknown_placeholders():
    out = P._fmt_workloads([WorkloadSignal(name="w", kind="Job")])
    assert "QoS=? cpu=? mem=?" in out


def test_build_prompt_full_contains_sections_and_fields():
    s = AppSignals(
        name="auth", namespace="telark", images=["nginx"], ports=[443],
        envVarKeys=["REDIS_HOST"], replicas=1, readyReplicas=1, healthStatus="healthy",
        workloadKinds=["Deployment"], hasService=True, secretRefs=["db"], managedBy="helm",
        chart="auth-1", incidents=2,
        workloads=[WorkloadSignal(name="api", kind="Deployment", replicas=1, qos="BestEffort",
                                  cpu="1m", memory="2Mi", limitsSet=False)],
    )
    out = P.build_prompt(s)
    for token in ("SIGNAL DICTIONARY", "CONFIDENCE RULES", "RESOURCE EFFICIENCY",
                  "CRITICALITY RULES", "TAGS RULES", "resourceEfficiency", "criticality"):
        assert token in out, token
    assert "auth (namespace: telark)" in out
    assert "Managed by: helm auth-1" in out
    assert "api Deployment x1" in out


def test_build_prompt_marks_empty_signals():
    out = P.build_prompt(AppSignals(name="bare", namespace="ns"))
    assert "(none provided)" in out
    assert "Health: unknown" in out


def test_prompt_version_stable_and_matches_module():
    v1 = P.get_prompt_version()
    v2 = P.get_prompt_version()
    assert v1 == v2 == P.PROMPT_VERSION
    assert len(v1) == 8 and all(c in "0123456789abcdef" for c in v1)
