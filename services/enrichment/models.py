"""Pydantic schemas for enrichment input and output."""

from datetime import datetime

from pydantic import BaseModel, Field

from constants import CATEGORY_PATTERN, CONFIDENCE_PATTERN


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


class RelatedApp(BaseModel):
    """Another application this app depends on (inferred from env keys)."""

    name: str
    reason: str


class EnrichmentResultLLM(BaseModel):
    """LLM response shape (no enrichedAt)."""

    summary: str
    techStack: list[str] = Field(default_factory=list)
    role: str                                              # free-form
    dependencies: list[str] = Field(default_factory=list)
    confidence: str = Field(..., pattern=CONFIDENCE_PATTERN)
    category: str = Field(..., pattern=CATEGORY_PATTERN)
    risks: list[str] = Field(default_factory=list)
    suggestions: list[str] = Field(default_factory=list)
    relatedApps: list[RelatedApp] = Field(default_factory=list)


class EnrichmentResult(BaseModel):
    """Structured enrichment result written to cache."""

    summary: str
    techStack: list[str] = Field(default_factory=list)
    role: str                                              # free-form
    dependencies: list[str] = Field(default_factory=list)
    confidence: str = Field(..., pattern=CONFIDENCE_PATTERN)
    enrichedAt: datetime
    category: str = Field(default="application")
    risks: list[str] = Field(default_factory=list)
    suggestions: list[str] = Field(default_factory=list)
    relatedApps: list[RelatedApp] = Field(default_factory=list)