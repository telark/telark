package plans

const (
	PhaseActive     = "active"
	PhaseCancelled  = "cancelled"
	PhaseDraft      = "draft"
	PhaseFailed     = "failed"
	PhaseScheduled  = "scheduled"
	PhaseTerminated = "terminated"

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
)

type ProtectionPlanScope struct {
	Type           string   `json:"type"`
	ApplicationIds []string `json:"applicationIds,omitempty"`
	Namespaces     []string `json:"namespaces,omitempty"`
}

type ProtectionPlanPolicy struct {
	TemplateID string         `json:"templateID"`
	Params     map[string]any `json:"params,omitempty"`
}

type ProtectionPlanTimeRange struct {
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}

type ProtectionPlan struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	Description      *string                 `json:"description,omitempty"`
	Severity         string                  `json:"severity"`
	Priority         int                     `json:"priority"`
	Scope            ProtectionPlanScope     `json:"scope"`
	Policies         []ProtectionPlanPolicy  `json:"policies"`
	Mode             string                  `json:"mode"`
	TimeMode         string                  `json:"timeMode"`
	TimeRange        *ProtectionPlanTimeRange `json:"timeRange,omitempty"`
	Phase            string                  `json:"phase"`
	Reason           *string                 `json:"reason,omitempty"`
	RenderedPolicies []string                `json:"renderedPolicies,omitempty"`
	CreatedAt        string                  `json:"createdAt"`
	CreatedBy        string                  `json:"createdBy"`
	LastUpdatedAt    string                  `json:"lastUpdatedAt"`
	LastUpdatedBy    string                  `json:"lastUpdatedBy"`
	StartedAt        *string                 `json:"startedAt,omitempty"`
	StartedBy        *string                 `json:"startedBy,omitempty"`
	TerminatedAt     *string                 `json:"terminatedAt,omitempty"`
	TerminatedBy     *string                 `json:"terminatedBy,omitempty"`
	ParticipantsIDs  []string                `json:"participantsIDs"`
}
