package plans

// CreateProtectionPlanRequest carries the full plan payload from discovery to exporter on create.
type CreateProtectionPlanRequest struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Description      *string               `json:"description,omitempty"`
	Severity         string                `json:"severity"`
	Priority         int                   `json:"priority"`
	Scope            ScopeRequest          `json:"scope"`
	Policies         []PolicyRequest       `json:"policies"`
	Mode             string                `json:"mode"`
	TimeMode         string                `json:"timeMode"`
	TimeRange        *TimeRangeRequest     `json:"timeRange,omitempty"`
	Phase            string                `json:"phase"`
	Reason           *string               `json:"reason,omitempty"`
	RenderedPolicies []string              `json:"renderedPolicies,omitempty"`
	CreatedAt        string                `json:"createdAt"`
	CreatedBy        string                `json:"createdBy"`
	LastUpdatedAt    string                `json:"lastUpdatedAt"`
	LastUpdatedBy    string                `json:"lastUpdatedBy"`
	StartedAt        *string               `json:"startedAt,omitempty"`
	StartedBy        *string               `json:"startedBy,omitempty"`
	TerminatedAt     *string               `json:"terminatedAt,omitempty"`
	TerminatedBy     *string               `json:"terminatedBy,omitempty"`
	ParticipantsIDs  []string              `json:"participantsIDs"`
	Health           string                `json:"health"`
	HealthCheckedAt  *string               `json:"healthCheckedAt,omitempty"`
	HealthDetail     []HealthDetailRequest `json:"healthDetail,omitempty"`
}

type HealthDetailRequest struct {
	PolicyName    string `json:"policyName"`
	Namespace     string `json:"namespace"`
	Present       bool   `json:"present"`
	Ready         bool   `json:"ready"`
	FailureAction string `json:"failureAction"`
}

// PatchProtectionPlanRequest carries only the mutable fields a client may update.
type PatchProtectionPlanRequest struct {
	Name             *string               `json:"name,omitempty"`
	Description      *string               `json:"description,omitempty"`
	Severity         *string               `json:"severity,omitempty"`
	Priority         *int                  `json:"priority,omitempty"`
	Mode             *string               `json:"mode,omitempty"`
	TimeMode         *string               `json:"timeMode,omitempty"`
	TimeRange        *TimeRangeRequest     `json:"timeRange,omitempty"`
	Scope            *ScopeRequest         `json:"scope,omitempty"`
	Policies         []PolicyRequest       `json:"policies,omitempty"`
	ParticipantsIDs  []string              `json:"participantsIDs,omitempty"`
	Phase            *string               `json:"phase,omitempty"`
	Reason           *string               `json:"reason,omitempty"`
	RenderedPolicies []string              `json:"renderedPolicies,omitempty"`
	StartedAt        *string               `json:"startedAt,omitempty"`
	StartedBy        *string               `json:"startedBy,omitempty"`
	TerminatedAt     *string               `json:"terminatedAt,omitempty"`
	TerminatedBy     *string               `json:"terminatedBy,omitempty"`
	LastUpdatedAt    *string               `json:"lastUpdatedAt,omitempty"`
	LastUpdatedBy    *string               `json:"lastUpdatedBy,omitempty"`
	Health           *string               `json:"health,omitempty"`
	HealthCheckedAt  *string               `json:"healthCheckedAt,omitempty"`
	HealthDetail     []HealthDetailRequest `json:"healthDetail,omitempty"`
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

type ReactivateProtectionPlanRequest struct {
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

type ProtectionPlanStatusResponse struct {
	PlanID   string                       `json:"planId"`
	Phase    string                       `json:"phase"`
	Health   string                       `json:"health"`
	Policies []ProtectionPlanPolicyStatus `json:"policies"`
	Drift    ProtectionPlanDrift          `json:"drift"`
}

type ProtectionPlanPolicyStatus struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Present       bool   `json:"present"`
	Ready         bool   `json:"ready"`
	FailureAction string `json:"failureAction"`
}

type ProtectionPlanDrift struct {
	Missing    []string `json:"missing"`
	Unexpected []string `json:"unexpected"`
}

type ProtectionPlanViolationsResponse struct {
	PlanID     string                    `json:"planId"`
	Total      int                       `json:"total"`
	Violations []ProtectionPlanViolation `json:"violations"`
}

type ProtectionPlanViolation struct {
	Policy    string                 `json:"policy"`
	Rule      string                 `json:"rule"`
	Namespace string                 `json:"namespace"`
	Resource  ProtectionPlanResource `json:"resource"`
	Result    string                 `json:"result"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
}

type ProtectionPlanResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DuplicateProtectionPlanRequest struct {
	Name      *string           `json:"name,omitempty"`
	TimeMode  *string           `json:"timeMode,omitempty"`
	TimeRange *TimeRangeRequest `json:"timeRange,omitempty"`
}
