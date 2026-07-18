"""Full coverage for providers/cache.py — fingerprint stability, volatility exclusion, eviction.

Run: pytest tests/test_cache_cov.py
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from datetime import datetime  # noqa: E402

from models import AppSignals, EnrichmentResult, WorkloadSignal  # noqa: E402
from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION  # noqa: E402
from providers import cache as C  # noqa: E402


def _sig(**over):
    base = dict(
        name="a", namespace="ns", images=["nginx", "redis"], ports=[443, 80],
        envVarKeys=["B", "A"], resourceKinds=["Service", "Deployment"],
        hasIngress=True, hasPVC=False, replicas=2, hasService=True, hasHPA=False,
        hasNetworkPolicy=True, secretRefs=["y", "x"], configMapRefs=["c"],
        managedBy="helm", chart="c-1", workloadKinds=["Deployment"],
        workloads=[WorkloadSignal(name="api", kind="Deployment", qos="Burstable",
                                  cpu="10m", memory="5Mi", limitsSet=True)],
    )
    base.update(over)
    return AppSignals(**base)


def test_hash_stable_and_order_independent():
    a = C.signals_hash(_sig())
    b = C.signals_hash(_sig(images=["redis", "nginx"], ports=[80, 443],
                            envVarKeys=["A", "B"], secretRefs=["x", "y"]))
    assert a == b
    assert len(a) == 16


def test_hash_excludes_volatile_signals():
    base = C.signals_hash(_sig())
    for over in (
        {"readyReplicas": 9}, {"healthStatus": "down"}, {"changeVelocityPerDay": 99.0},
        {"incidents": 42}, {"recoveries": 7},
        {"workloads": [WorkloadSignal(name="api", kind="Deployment", qos="Burstable",
                                      cpu="999m", memory="9Gi", limitsSet=True)]},
    ):
        assert C.signals_hash(_sig(**over)) == base, over


def test_hash_includes_structural_signals():
    base = C.signals_hash(_sig())
    for over in (
        {"replicas": 5}, {"hasHPA": True}, {"hasNetworkPolicy": False},
        {"secretRefs": ["z"]}, {"managedBy": "kustomize"}, {"chart": "other"},
        {"workloadKinds": ["StatefulSet"]},
        {"workloads": [WorkloadSignal(name="api", kind="StatefulSet", qos="Burstable",
                                      cpu="10m", memory="5Mi", limitsSet=True)]},
        {"workloads": [WorkloadSignal(name="api", kind="Deployment", qos="BestEffort",
                                      cpu="10m", memory="5Mi", limitsSet=True)]},
        {"workloads": [WorkloadSignal(name="api", kind="Deployment", qos="Burstable",
                                      cpu="10m", memory="5Mi", limitsSet=False)]},
    ):
        assert C.signals_hash(_sig(**over)) != base, over


def test_cache_key_binds_prompt_version():
    assert C.cache_key("abc") == f"abc:{PROMPT_VERSION}"


def test_get_set_and_eviction():
    C._RESPONSE_CACHE.clear()
    assert C.get_cached("missing") is None
    r = EnrichmentResult(summary="s", role="r", confidence="low", enrichedAt=datetime.utcnow())
    C.set_cached("k0", r)
    assert C.get_cached("k0") is r
    # overflow the cache; oldest ("k0") gets evicted
    for i in range(C._CACHE_MAX_SIZE + 5):
        C.set_cached(f"k{i}", r)
    assert C.get_cached("k0") is None
    assert len(C._RESPONSE_CACHE) <= C._CACHE_MAX_SIZE
    C._RESPONSE_CACHE.clear()
