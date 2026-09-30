package plans

import globalshared "github.com/telark/data/shared"

const (
	PhaseActive          = "active"
	PhaseCanceled        = "canceled"
	PhaseDraft           = "draft"
	PhaseFailed          = "failed"
	PhasePendingApproval = "pending_approval"
	PhaseScheduled       = "scheduled"
	PhaseTerminated      = "terminated"

	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityLow      = "low"
	SeverityMedium   = "medium"

	ModeAudit   = "audit"
	ModeEnforce = "enforce"

	TimeModePermanent = "permanent"
	TimeModeTimeRange = "time_range"

	ScopeTypeApplications = "applications"
	ScopeTypeNamespaces   = "namespaces"

	HealthUnknown  = "unknown"
	HealthHealthy  = "healthy"
	HealthDrifted  = "drifted"
	HealthDegraded = "degraded"

	ApprovalModeAutomatic = "automatic"
	ApprovalModeRequired  = "required"

	ApprovalStatePending  = "pending"
	ApprovalStateApproved = "approved"
	ApprovalStateRejected = "rejected"

	ApprovalEventRequested = "requested"
	ApprovalEventApproved  = "approved"
	ApprovalEventRejected  = "rejected"

	ApprovalHistoryMax = 20

	ConditionTypeReady           = "Ready"
	ConditionTypeApproved        = "Approved"
	ConditionTypePoliciesHealthy = "PoliciesHealthy"

	ExclusionKindsMax           = 50
	ExclusionResourcesMax       = 200
	ExclusionKindMaxLength      = 63
	ExclusionNameMaxLength      = 253
	ExclusionNamespaceMaxLength = 63
	SubresourceSeparator        = "/"
)

type ProtectionPlanScope struct {
	Type            string                         `json:"type"`
	ApplicationRefs []string                       `json:"applicationRefs,omitempty"`
	Namespaces      []string                       `json:"namespaces,omitempty"`
	Exclusions      *ProtectionPlanScopeExclusions `json:"exclusions,omitempty"`
}

type ProtectionPlanScopeExclusions struct {
	Kinds     []string                         `json:"kinds,omitempty"`
	Resources []ProtectionPlanExcludedResource `json:"resources,omitempty"`
}

type ProtectionPlanExcludedResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ProtectionPlanPolicy struct {
	TemplateID string         `json:"templateID"`
	Params     map[string]any `json:"params,omitempty"`
}

type ProtectionPlanTimeRange struct {
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}

type ProtectionPlanHealthDetail struct {
	PolicyName    string `json:"policyName"`
	Namespace     string `json:"namespace"`
	Present       bool   `json:"present"`
	Ready         bool   `json:"ready"`
	FailureAction string `json:"failureAction"`
}

type ProtectionPlanApprovalEvent struct {
	Event   string  `json:"event"`
	By      string  `json:"by"`
	At      string  `json:"at"`
	Comment *string `json:"comment,omitempty"`
}

type ProtectionPlanApproval struct {
	State       string                        `json:"state"`
	RequestedBy string                        `json:"requestedBy"`
	RequestedAt string                        `json:"requestedAt"`
	DecidedBy   *string                       `json:"decidedBy,omitempty"`
	DecidedAt   *string                       `json:"decidedAt,omitempty"`
	Comment     *string                       `json:"comment,omitempty"`
	History     []ProtectionPlanApprovalEvent `json:"history,omitempty"`
}

type ProtectionPlan struct {
	ID                 string                       `json:"id"`
	Name               string                       `json:"name"`
	Description        *string                      `json:"description,omitempty"`
	Severity           string                       `json:"severity"`
	Priority           int                          `json:"priority"`
	Scope              ProtectionPlanScope          `json:"scope"`
	Policies           []ProtectionPlanPolicy       `json:"policies"`
	Mode               string                       `json:"mode"`
	TimeMode           string                       `json:"timeMode"`
	TimeRange          *ProtectionPlanTimeRange     `json:"timeRange,omitempty"`
	Phase              string                       `json:"phase"`
	Reason             *string                      `json:"reason,omitempty"`
	RenderedPolicies   []string                     `json:"renderedPolicies,omitempty"`
	CreatedAt          string                       `json:"createdAt"`
	CreatedBy          string                       `json:"createdBy"`
	LastUpdatedAt      string                       `json:"lastUpdatedAt"`
	LastUpdatedBy      string                       `json:"lastUpdatedBy"`
	StartedAt          *string                      `json:"startedAt,omitempty"`
	StartedBy          *string                      `json:"startedBy,omitempty"`
	TerminatedAt       *string                      `json:"terminatedAt,omitempty"`
	TerminatedBy       *string                      `json:"terminatedBy,omitempty"`
	ParticipantRefs    []string                     `json:"participantRefs"`
	EnvironmentRef     string                       `json:"environmentRef,omitempty"`
	TagRefs            []string                     `json:"tagRefs,omitempty"`
	ApprovalMode       string                       `json:"approvalMode,omitempty"`
	Approval           *ProtectionPlanApproval      `json:"approval,omitempty"`
	Health             string                       `json:"health"`
	HealthCheckedAt    *string                      `json:"healthCheckedAt,omitempty"`
	HealthDetail       []ProtectionPlanHealthDetail `json:"healthDetail,omitempty"`
	Conditions         []globalshared.Condition     `json:"conditions,omitempty"`
	ObservedGeneration int64                        `json:"observedGeneration,omitempty"`
}
