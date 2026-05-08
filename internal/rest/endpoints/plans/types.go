package plans

// CreateProtectionPlanRequest carries the full plan payload from discovery to exporter on create.
type CreateProtectionPlanRequest struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Description      *string           `json:"description,omitempty"`
	Severity         string            `json:"severity"`
	Priority         int               `json:"priority"`
	Scope            ScopeRequest      `json:"scope"`
	Policies         []PolicyRequest   `json:"policies"`
	Mode             string            `json:"mode"`
	TimeMode         string            `json:"timeMode"`
	TimeRange        *TimeRangeRequest `json:"timeRange,omitempty"`
	Phase            string            `json:"phase"`
	Reason           *string           `json:"reason,omitempty"`
	RenderedPolicies []string          `json:"renderedPolicies,omitempty"`
	CreatedAt        string            `json:"createdAt"`
	CreatedBy        string            `json:"createdBy"`
	LastUpdatedAt    string            `json:"lastUpdatedAt"`
	LastUpdatedBy    string            `json:"lastUpdatedBy"`
	StartedAt        *string           `json:"startedAt,omitempty"`
	StartedBy        *string           `json:"startedBy,omitempty"`
	TerminatedAt     *string           `json:"terminatedAt,omitempty"`
	TerminatedBy     *string           `json:"terminatedBy,omitempty"`
	ParticipantsIDs  []string          `json:"participantsIDs"`
}

// PatchProtectionPlanRequest carries only the mutable fields a client may update.
type PatchProtectionPlanRequest struct {
	Name             *string  `json:"name,omitempty"`
	Description      *string  `json:"description,omitempty"`
	Severity         *string  `json:"severity,omitempty"`
	Priority         *int     `json:"priority,omitempty"`
	Phase            *string  `json:"phase,omitempty"`
	Reason           *string  `json:"reason,omitempty"`
	RenderedPolicies []string `json:"renderedPolicies,omitempty"`
	StartedAt        *string  `json:"startedAt,omitempty"`
	StartedBy        *string  `json:"startedBy,omitempty"`
	TerminatedAt     *string  `json:"terminatedAt,omitempty"`
	TerminatedBy     *string  `json:"terminatedBy,omitempty"`
	LastUpdatedAt    *string  `json:"lastUpdatedAt,omitempty"`
	LastUpdatedBy    *string  `json:"lastUpdatedBy,omitempty"`
}

// PrepareProtectionPlanRequest is the user-facing payload sent by the UI to discovery.
// No id, no audit fields, no phase — those are all derived or set server-side.
type PrepareProtectionPlanRequest struct {
	Name            string            `json:"name"`
	Description     *string           `json:"description,omitempty"`
	Severity        string            `json:"severity"`
	Priority        int               `json:"priority"`
	Scope           ScopeRequest      `json:"scope"`
	Policies        []PolicyRequest   `json:"policies"`
	Mode            string            `json:"mode"`
	TimeMode        string            `json:"timeMode"`
	TimeRange       *TimeRangeRequest `json:"timeRange,omitempty"`
	ParticipantsIDs []string          `json:"participantsIDs"`
}

type CancelProtectionPlanRequest struct {
	Reason *string `json:"reason,omitempty"`
}

type ScopeRequest struct {
	Type           string   `json:"type"`
	ApplicationIDs []string `json:"applicationIds,omitempty"`
	Namespaces     []string `json:"namespaces,omitempty"`
}

type PolicyRequest struct {
	TemplateID string         `json:"templateID"`
	Params     map[string]any `json:"params,omitempty"`
}

type TimeRangeRequest struct {
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}
