"""Pydantic mirrors of the frozen Go contract; JSON names equal the Go json tags.

internal/data/resources/application/insights.go:42-81 (the insight document) and
internal/data/insights/types.go:16-53 (jobs, runtime, validation). Unknown input
fields are ignored, like Go's decoder.
"""

import asyncio
from collections import Counter
from dataclasses import dataclass, field
from typing import ClassVar, Literal

from pydantic import BaseModel, ConfigDict, Field, model_serializer

from constants import (
    CONFIDENCE_HIGH,
    CONFIDENCE_LOW,
    CONFIDENCE_MEDIUM,
    DEFAULT_ANALYZER_MODEL,
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
    MAX_EVIDENCE_PER_INSIGHT,
    MAX_INSIGHTS_PER_RUN,
    NARRATE_SUMMARY_MAX,
    NARRATE_TITLE_MAX,
    SSE_QUEUE_MAX,
    SUBJECT_TEMPLATE,
    TRIGGER_INCIDENT,
    TRIGGER_MANUAL,
    TRIAGE_ACTION_ACKNOWLEDGE,
    TRIAGE_ACTION_DISMISS,
    TRIAGE_ACTION_REOPEN,
    TRIGGER_RECOVERY,
    WORKLOAD_DAEMONSET,
    WORKLOAD_DEPLOYMENT,
    WORKLOAD_KINDS,
    WORKLOAD_STATEFULSET,
)


class _OmitEmpty(BaseModel):
    """Drops the listed fields when empty, as Go's `omitempty` does."""

    _omit_empty: ClassVar[tuple[str, ...]] = ()

    @model_serializer(mode="wrap")
    def _drop_empty(self, handler):
        data = handler(self)
        for key in self._omit_empty:
            if not data.get(key):
                data.pop(key, None)
        return data


class EvidenceRef(BaseModel):
    type: str = ""
    ref: str = ""


class InsightTriage(BaseModel):
    state: str = ""
    by: str = ""
    at: str = ""


class Insight(_OmitEmpty):
    _omit_empty: ClassVar[tuple[str, ...]] = ("resolvedAt", "category", "reason", "params", "triage")

    id: str = ""
    kind: str = ""
    subject: str = ""
    title: str = ""
    summary: str = ""
    confidence: str = ""
    severity: str = ""
    status: str = ""
    evidence: list[EvidenceRef] = Field(default_factory=list)
    firstSeenAt: str = ""
    lastSeenAt: str = ""
    resolvedAt: str = ""
    runs: int = 0
    category: str = ""
    reason: str = ""
    params: dict[str, str] = Field(default_factory=dict)
    triage: InsightTriage | None = None


class LastRun(_OmitEmpty):
    _omit_empty: ClassVar[tuple[str, ...]] = ("queuedAt",)

    status: str = ""
    trigger: str = ""
    runId: str = ""
    queuedAt: str = ""
    startedAt: str = ""
    finishedAt: str = ""
    error: str = ""
    model: str = ""
    steps: int = 0
    toolCalls: int = 0
    truncated: bool = False


class AppInsights(_OmitEmpty):
    _omit_empty: ClassVar[tuple[str, ...]] = ("lastReviewAt",)

    insights: list[Insight] = Field(default_factory=list)
    lastRun: LastRun = Field(default_factory=LastRun)
    version: int = 0
    lastReviewAt: str = ""


class Job(BaseModel):
    namespace: str
    name: str
    trigger: Literal[TRIGGER_MANUAL, TRIGGER_INCIDENT, TRIGGER_RECOVERY]
    generation: int


class TriageRequest(BaseModel):
    action: Literal[TRIAGE_ACTION_ACKNOWLEDGE, TRIAGE_ACTION_DISMISS, TRIAGE_ACTION_REOPEN]


class AnalyzeResponse(BaseModel):
    runId: str
    status: str


class PullProgress(BaseModel):
    model: str = ""
    status: str = ""
    completed: int = 0
    total: int = 0


class RuntimeStatus(_OmitEmpty):
    _omit_empty: ClassVar[tuple[str, ...]] = ("pull",)

    state: str = ""
    model: str = ""
    reason: str = ""
    mode: str = ""
    autoPull: bool = False
    pull: PullProgress | None = None


class ValidateModelRequest(BaseModel):
    model: str


class ValidateModelResponse(BaseModel):
    ok: bool = False
    model: str = ""
    license: str = ""
    warning: str = ""
    reason: str = ""
    capabilities: list[str] = Field(default_factory=list)


class AnalyzerConfig(BaseModel):
    """GlobalConfig spec.ai plus excludedNamespaces, as the analyzer needs them."""

    enabled: bool = False
    model: str = DEFAULT_ANALYZER_MODEL
    autoAnalyze: bool = False
    excludedNamespaces: list[str] = Field(default_factory=list)


class ToolCallFunction(BaseModel):
    name: str
    arguments: dict = Field(default_factory=dict)


class ToolCall(BaseModel):
    function: ToolCallFunction


class ChatMessage(_OmitEmpty):
    """One Ollama /api/chat message."""

    _omit_empty: ClassVar[tuple[str, ...]] = ("tool_calls", "tool_name")

    role: str
    content: str = ""
    tool_calls: list[ToolCall] = Field(default_factory=list)
    tool_name: str = ""


class ChatResponse(BaseModel):
    """A non-streaming Ollama /api/chat answer; unknown fields are ignored."""

    message: ChatMessage
    done: bool
    done_reason: str = ""
    prompt_eval_count: int = 0
    eval_count: int = 0


class _ToolArgs(BaseModel):
    model_config = ConfigDict(extra="forbid")


class OverviewArgs(_ToolArgs):
    pass


class HistoryArgs(_ToolArgs):
    limit: int = Field(5, ge=1, le=10)
    onlyIncidents: bool = False


class WorkloadArgs(_ToolArgs):
    kind: Literal[WORKLOAD_DEPLOYMENT, WORKLOAD_STATEFULSET, WORKLOAD_DAEMONSET]
    name: str
    # Empty = the first app namespace that has the workload.
    namespace: str = ""


class EventsArgs(_ToolArgs):
    sinceMinutes: int = Field(30, ge=5, le=120)
    warningsOnly: bool = True
    # Empty = every app namespace that is not excluded.
    namespace: str = ""


class ToolResult(BaseModel):
    name: str
    content: str
    truncated: bool = False
    refs: list[str] = Field(default_factory=list)
    error: str = ""


@dataclass
class Run:
    """One analysis run's tool state; `app` is the exporter's Application JSON (def.go:3-125)."""

    namespace: str
    name: str
    app: dict
    excluded: list[str]
    # (namespace, '<kind>/<name>') -> (kind, name, namespace); workloads in excluded namespaces never bind.
    workloads: dict[tuple[str, str], tuple[str, str, str]] = field(init=False)
    refs: set[str] = field(default_factory=set)
    # The caches are keyed like workloads: one app may run the same workload in two namespaces.
    status_cache: dict[tuple[str, str], dict] = field(default_factory=dict)
    events_cache: list[dict] = field(default_factory=list)
    # The status reads' raw workload objects and pod items, reused by the review (None = pods not listed).
    spec_cache: dict[tuple[str, str], dict] = field(default_factory=dict)
    pods_cache: dict[tuple[str, str], list[dict] | None] = field(default_factory=dict)
    # Workload keys whose status read answered 404: deleted, though the Application still lists them.
    gone: set[tuple[str, str]] = field(default_factory=set)
    calls: Counter = field(default_factory=Counter)
    tool_calls: int = 0
    truncated: bool = False

    def __post_init__(self) -> None:
        self.workloads = {}
        for res in self.app.get("resources") or []:
            kind = (res.get("kind") or "").lower()
            if kind in WORKLOAD_KINDS and res.get("namespace") not in self.excluded:
                subject = SUBJECT_TEMPLATE.format(kind=kind, name=res.get("name"))
                self.workloads[(res.get("namespace"), subject)] = (kind, res.get("name"), res.get("namespace"))

    def bind(self, subject: str, namespace: str = "") -> tuple[str, str] | None:
        """The workload key of a subject: in `namespace` when given, else in the first app namespace that has it."""
        return next((key for key in self.workloads if key[1] == subject and (not namespace or key[0] == namespace)),
                    None)


class Emitted(BaseModel):
    """One insight as the model emits it (the EMIT decode target); the analyzer stamps the rest."""

    model_config = ConfigDict(extra="forbid")

    kind: Literal[
        INSIGHT_KIND_CRASHLOOP,
        INSIGHT_KIND_OOM,
        INSIGHT_KIND_IMAGE_PULL,
        INSIGHT_KIND_PROBE_FAILURE,
        INSIGHT_KIND_SCHEDULING,
        INSIGHT_KIND_ROLLOUT_STUCK,
        INSIGHT_KIND_CONFIG_CHANGE_REGRESSION,
        INSIGHT_KIND_RESOURCE_PRESSURE,
        INSIGHT_KIND_OTHER,
    ]
    subject: str
    title: str
    summary: str
    confidence: Literal[CONFIDENCE_LOW, CONFIDENCE_MEDIUM, CONFIDENCE_HIGH]
    severity: Literal[INSIGHT_SEVERITY_INFO, INSIGHT_SEVERITY_WARNING, INSIGHT_SEVERITY_CRITICAL]
    evidence: list[EvidenceRef] = Field(max_length=MAX_EVIDENCE_PER_INSIGHT)
    # Set by the rules only (fast mode); a deep-mode insight carries none and the UI uses the kind's text.
    # insights.validate adds params["namespace"], the workload's namespace, in both modes.
    reason: str = ""
    params: dict[str, str] = Field(default_factory=dict)


class EmitOutput(BaseModel):
    """The strict EMIT decode target."""

    model_config = ConfigDict(extra="forbid")

    insights: list[Emitted] = Field(max_length=MAX_INSIGHTS_PER_RUN)


@dataclass
class Candidate:
    """One rule-detected insight: code decides every field; `facts` are the only input the narration may phrase."""

    subject: str
    kind: str
    severity: str
    confidence: str
    evidence: list[EvidenceRef]
    title: str
    summary: str
    facts: list[str]
    reason: str = ""
    params: dict[str, str] = field(default_factory=dict)

    def to_emitted(self) -> Emitted:
        return Emitted(kind=self.kind, subject=self.subject, title=self.title, summary=self.summary,
                       confidence=self.confidence, severity=self.severity, evidence=self.evidence,
                       reason=self.reason, params=self.params)


class Narrated(BaseModel):
    model_config = ConfigDict(extra="forbid")

    title: str = Field(max_length=NARRATE_TITLE_MAX)
    summary: str = Field(max_length=NARRATE_SUMMARY_MAX)


class NarrateOutput(BaseModel):
    """The strict narration decode target: one item per candidate, in order."""

    model_config = ConfigDict(extra="forbid")

    insights: list[Narrated]


@dataclass
class Outcome:
    """One analysis for the worker to persist; `error` is a lastRun.error code, never an exception message."""

    emitted: list[Emitted] = field(default_factory=list)
    truncated: bool = False
    steps: int = 0
    tool_calls: int = 0
    error: str = ""


@dataclass
class MergeStats:
    """Insight ids merge changed this run; `touched` is all of them (resolve_observed skips it)."""

    created: list[str] = field(default_factory=list)
    updated: list[str] = field(default_factory=list)
    reopened: list[str] = field(default_factory=list)
    touched: set[str] = field(default_factory=set)


@dataclass
class RecStats:
    """Recommendation ids one review changed."""

    created: list[str] = field(default_factory=list)
    updated: list[str] = field(default_factory=list)
    reopened: list[str] = field(default_factory=list)
    resolved: list[str] = field(default_factory=list)


@dataclass(eq=False)
class Subscription:
    """One SSE connection: the apps it may see and its bounded queue of (event name, data)."""

    apps: set[str]
    queue: asyncio.Queue = field(default_factory=lambda: asyncio.Queue(maxsize=SSE_QUEUE_MAX))


def to_json(model: BaseModel) -> str:
    """The one serializer for every Redis document write."""
    return model.model_dump_json()
