"""Pydantic schemas for enrichment input and output."""

from datetime import datetime

from pydantic import BaseModel, Field

from constants import (
    CATEGORY_PATTERN,
    CONFIDENCE_PATTERN,
    CRITICALITY_LEVEL_PATTERN,
    EFFICIENCY_STATUS_PATTERN,
    PRIORITY_PATTERN,
    SEVERITY_PATTERN,
)


class WorkloadSignal(BaseModel):
    """Compact per-workload usage/posture, grounded from the Application CRD."""

    name: str
    kind: str
    replicas: int = 0
    qos: str = ""
    cpu: str = ""
    memory: str = ""
    limitsSet: bool = False


class AppSignals(BaseModel):
    """Kubernetes application signals consumed from the queue."""

    name: str
    namespace: str
    images: list[str] = Field(default_factory=list)
    ports: list[int] = Field(default_factory=list)
    envVarKeys: list[str] = Field(default_factory=list)
    resourceKinds: list[str] = Field(default_factory=list)
    hasIngress: bool = False
    hasPVC: bool = False

    replicas: int = 0
    readyReplicas: int = 0
    healthStatus: str = ""

    workloadKinds: list[str] = Field(default_factory=list)
    hasService: bool = False
    hasHPA: bool = False
    hasNetworkPolicy: bool = False

    secretRefs: list[str] = Field(default_factory=list)
    configMapRefs: list[str] = Field(default_factory=list)

    managedBy: str = ""
    chart: str = ""

    changeVelocityPerDay: float = 0.0
    incidents: int = 0
    recoveries: int = 0

    workloads: list[WorkloadSignal] = Field(default_factory=list)


class InsightsDispatchRequest(BaseModel):
    """A whole scope's signals in one call, so the fan-out never crosses the wire
    per app."""

    items: list[AppSignals] = Field(default_factory=list)


class InsightsDispatchResponse(BaseModel):
    """Names only. Ready are cached now; pending were queued and fill in on later
    polls. Discovery reads the results themselves from the cache, not from here."""

    ready: list[str] = Field(default_factory=list)
    pending: list[str] = Field(default_factory=list)


class RelatedApp(BaseModel):
    """Another application this app depends on (inferred from env keys)."""

    name: str
    reason: str


class Risk(BaseModel):
    severity: str = Field(..., pattern=SEVERITY_PATTERN)
    message: str


class Suggestion(BaseModel):
    priority: str = Field(..., pattern=PRIORITY_PATTERN)
    message: str


class ResourceEfficiency(BaseModel):
    status: str = Field(..., pattern=EFFICIENCY_STATUS_PATTERN)
    note: str = ""


class Criticality(BaseModel):
    level: str = Field(..., pattern=CRITICALITY_LEVEL_PATTERN)
    reason: str = ""


def _default_efficiency() -> ResourceEfficiency:
    return ResourceEfficiency(status="unknown", note="")


def _default_criticality() -> Criticality:
    return Criticality(level="low", reason="")


class EnrichmentResultLLM(BaseModel):
    """LLM response shape (no enrichedAt). resourceEfficiency and criticality are
    required so the structured-output layer forces the model to assess them."""

    summary: str
    techStack: list[str] = Field(default_factory=list)
    role: str                                              # free-form
    dependencies: list[str] = Field(default_factory=list)
    confidence: str = Field(..., pattern=CONFIDENCE_PATTERN)
    category: str = Field(..., pattern=CATEGORY_PATTERN)
    risks: list[Risk] = Field(default_factory=list)
    suggestions: list[Suggestion] = Field(default_factory=list)
    resourceEfficiency: ResourceEfficiency
    criticality: Criticality
    tags: list[str] = Field(default_factory=list)
    relatedApps: list[RelatedApp] = Field(default_factory=list)


class EnrichmentResult(BaseModel):
    """Structured enrichment result written to cache. Defaults on the assessment
    fields keep the fallback path simple."""

    summary: str
    techStack: list[str] = Field(default_factory=list)
    role: str                                              # free-form
    dependencies: list[str] = Field(default_factory=list)
    confidence: str = Field(..., pattern=CONFIDENCE_PATTERN)
    enrichedAt: datetime
    category: str = Field(default="application")
    risks: list[Risk] = Field(default_factory=list)
    suggestions: list[Suggestion] = Field(default_factory=list)
    resourceEfficiency: ResourceEfficiency = Field(default_factory=_default_efficiency)
    criticality: Criticality = Field(default_factory=_default_criticality)
    tags: list[str] = Field(default_factory=list)
    relatedApps: list[RelatedApp] = Field(default_factory=list)
