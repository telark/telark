package application

const (
	InsightKindCrashloop              = "crashloop"
	InsightKindOOM                    = "oom"
	InsightKindImagePull              = "image_pull"
	InsightKindProbeFailure           = "probe_failure"
	InsightKindScheduling             = "scheduling"
	InsightKindRolloutStuck           = "rollout_stuck"
	InsightKindConfigChangeRegression = "config_change_regression"
	InsightKindResourcePressure       = "resource_pressure"
	InsightKindOther                  = "other"

	InsightCategoryIncident       = "incident"
	InsightCategoryRecommendation = "recommendation"

	RecommendationKindReliability = "reliability"
	RecommendationKindResources   = "resources"
	RecommendationKindScaling     = "scaling"
	RecommendationKindSecurity    = "security"
	RecommendationKindImages      = "images"
	RecommendationKindConfig      = "config"
	RecommendationKindNetworking  = "networking"
	RecommendationKindChangeRisk  = "change_risk"
	RecommendationKindProtection  = "protection"
	RecommendationKindConsistency = "consistency"

	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"

	// Prefixed: SeverityCritical already names the change-log severity in def.go.
	InsightSeverityInfo     = "info"
	InsightSeverityWarning  = "warning"
	InsightSeverityCritical = "critical"

	InsightStatusOpen     = "open"
	InsightStatusUpdated  = "updated"
	InsightStatusResolved = "resolved"

	RunStatusQueued  = "queued"
	RunStatusRunning = "running"
	RunStatusDone    = "done"
	RunStatusFailed  = "failed"

	EvidenceTypeChange   = "change"
	EvidenceTypeSnapshot = "snapshot"
	EvidenceTypeEvent    = "event"
	EvidenceTypeWorkload = "workload"
	EvidenceTypeSpec     = "spec"
	EvidenceTypeObject   = "object"
	EvidenceTypeMetric   = "metric"
	EvidenceTypePlan     = "plan"

	TriageStateAcknowledged = "acknowledged"
	TriageStateDismissed    = "dismissed"

	TriageActionAcknowledge = "acknowledge"
	TriageActionDismiss     = "dismiss"
	TriageActionReopen      = "reopen"

	MaxInsightTitleLength    = 120
	MaxInsightSummaryLength  = 400
	MaxInsightsPerRun        = 3
	MaxRecommendationsPerApp = 40
	MaxInsightParams         = 16
	MaxInsightParamLength    = 120
)

type AppInsights struct {
	Insights     []Insight `json:"insights"`
	LastRun      LastRun   `json:"lastRun"`
	Version      int       `json:"version"`
	LastReviewAt string    `json:"lastReviewAt,omitempty"`
}

type Insight struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"`
	Subject     string            `json:"subject"`
	Title       string            `json:"title"`
	Summary     string            `json:"summary"`
	Confidence  string            `json:"confidence"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Evidence    []EvidenceRef     `json:"evidence"`
	FirstSeenAt string            `json:"firstSeenAt"`
	LastSeenAt  string            `json:"lastSeenAt"`
	ResolvedAt  string            `json:"resolvedAt,omitempty"`
	Runs        int               `json:"runs"`
	Category    string            `json:"category,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
	Triage      *InsightTriage    `json:"triage,omitempty"`
}

type InsightTriage struct {
	State string `json:"state"`
	By    string `json:"by"`
	At    string `json:"at"`
}

type EvidenceRef struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

type LastRun struct {
	Status     string `json:"status"`
	Trigger    string `json:"trigger"`
	RunID      string `json:"runId"`
	QueuedAt   string `json:"queuedAt,omitempty"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Error      string `json:"error"`
	Model      string `json:"model"`
	Steps      int    `json:"steps"`
	ToolCalls  int    `json:"toolCalls"`
	Truncated  bool   `json:"truncated"`
}
