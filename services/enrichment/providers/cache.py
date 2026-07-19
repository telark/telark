from __future__ import annotations

"""Shared in-process signal-based response cache for all providers."""

import hashlib
import json

from models import AppSignals, EnrichmentResult
from prompts.k8s_app_analyzer_prompt import PROMPT_VERSION


_RESPONSE_CACHE: dict[str, EnrichmentResult] = {}
_CACHE_MAX_SIZE = 512


def signals_hash(signals: AppSignals) -> str:
    """
    Compute a stable hash from enrichment-relevant signals only.
    Name and namespace excluded — they don't affect analysis.
    Same tech signals = same hash = same result = zero LLM call.
    """
    # Structural identity only. Volatile metrics (actual CPU/mem usage,
    # readyReplicas, health, change velocity, incidents) are deliberately excluded
    # so momentary drift doesn't re-trigger an LLM call every tick. Configuration
    # posture that does change the analysis (replicas, limits set, QoS, workload
    # kinds, wiring) is included.
    fingerprint = {
        "images": sorted(signals.images),
        "ports": sorted(signals.ports),
        "envVarKeys": sorted(signals.envVarKeys),
        "resourceKinds": sorted(signals.resourceKinds),
        "hasIngress": signals.hasIngress,
        "hasPVC": signals.hasPVC,
        "replicas": signals.replicas,
        "workloadKinds": sorted(signals.workloadKinds),
        "hasService": signals.hasService,
        "hasHPA": signals.hasHPA,
        "hasNetworkPolicy": signals.hasNetworkPolicy,
        "secretRefs": sorted(signals.secretRefs),
        "configMapRefs": sorted(signals.configMapRefs),
        "managedBy": signals.managedBy,
        "chart": signals.chart,
        "workloadSpec": sorted(
            f"{w.kind}:{w.qos}:{w.limitsSet}" for w in signals.workloads
        ),
    }
    raw = json.dumps(fingerprint, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(raw.encode()).hexdigest()[:16]


def cache_key(sig_hash: str) -> str:
    """Bind cache entry to current prompt version — auto-invalidates on prompt change."""
    return f"{sig_hash}:{PROMPT_VERSION}"


def get_cached(key: str) -> EnrichmentResult | None:
    return _RESPONSE_CACHE.get(key)


def set_cached(key: str, result: EnrichmentResult) -> None:
    if len(_RESPONSE_CACHE) >= _CACHE_MAX_SIZE:
        oldest = next(iter(_RESPONSE_CACHE))
        del _RESPONSE_CACHE[oldest]
    _RESPONSE_CACHE[key] = result