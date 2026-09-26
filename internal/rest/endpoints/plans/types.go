package plans

import "github.com/telark/data/plans"

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
	ParticipantRefs  []string              `json:"participantRefs"`
	EnvironmentRef   string                `json:"environmentRef,omitempty"`
	TagRefs          []string              `json:"tagRefs,omitempty"`
	ApprovalMode     string                `json:"approvalMode,omitempty"`
	Approval         *ApprovalRequest      `json:"approval,omitempty"`
	Health           string                `json:"health"`
	HealthCheckedAt  *string               `json:"healthCheckedAt,omitempty"`
	HealthDetail     []HealthDetailRequest `json:"healthDetail,omitempty"`
}

type ApprovalEventRequest struct {
	Event   string  `json:"event"`
	By      string  `json:"by"`
	At      string  `json:"at"`
	Comment *string `json:"comment,omitempty"`
}

type ApprovalRequest struct {
	State       string                 `json:"state"`
	RequestedBy string                 `json:"requestedBy"`
	RequestedAt string                 `json:"requestedAt"`
	DecidedBy   *string                `json:"decidedBy,omitempty"`
	DecidedAt   *string                `json:"decidedAt,omitempty"`
	Comment     *string                `json:"comment,omitempty"`
	History     []ApprovalEventRequest `json:"history,omitempty"`
}

type HealthDetailRequest struct {
	PolicyName    string `json:"policyName"`
	Namespace     string `json:"namespace"`
	Present       bool   `json:"present"`
	Ready         bool   `json:"ready"`
	FailureAction string `json:"failureAction"`
}

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
	ParticipantRefs  []string              `json:"participantRefs,omitempty"`
	EnvironmentRef   *string               `json:"environmentRef,omitempty"`
	TagRefs          *[]string             `json:"tagRefs,omitempty"`
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
	ParticipantRefs []string          `json:"participantRefs"`
	EnvironmentRef  *string           `json:"environmentRef,omitempty"`
	TagRefs         []string          `json:"tagRefs,omitempty"`
	ApprovalMode    *string           `json:"approvalMode,omitempty"`
}

type CancelProtectionPlanRequest struct {
	Reason *string `json:"reason,omitempty"`
}

type ReactivateProtectionPlanRequest struct {
	Reason *string `json:"reason,omitempty"`
}

type DecideProtectionPlanRequest struct {
	Decision    string  `json:"decision"`
	Comment     *string `json:"comment,omitempty"`
	RequestedAt string  `json:"requestedAt"`
}

type ScopeRequest struct {
	Type            string                               `json:"type"`
	ApplicationRefs []string                             `json:"applicationRefs,omitempty"`
	Namespaces      []string                             `json:"namespaces,omitempty"`
	Exclusions      *plans.ProtectionPlanScopeExclusions `json:"exclusions,omitempty"`
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

// RetentionWindow tells the caller how far back the underlying record reaches, so an
// empty Violations list is read as "nothing blocked in this window" and not "nothing ever".
type ProtectionPlanViolationsResponse struct {
	PlanID          string                    `json:"planId"`
	Total           int                       `json:"total"`
	RetentionWindow string                    `json:"retentionWindow"`
	Violations      []ProtectionPlanViolation `json:"violations"`
}

type ProtectionPlanViolation struct {
	Policy    string                 `json:"policy"`
	Rule      string                 `json:"rule"`
	Namespace string                 `json:"namespace"`
	Resource  ProtectionPlanResource `json:"resource"`
	Result    string                 `json:"result"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	EventUID  string                 `json:"eventUID,omitempty"`
}

type ProtectionPlanResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DuplicateProtectionPlanRequest struct {
	Name           *string           `json:"name,omitempty"`
	TimeMode       *string           `json:"timeMode,omitempty"`
	TimeRange      *TimeRangeRequest `json:"timeRange,omitempty"`
	EnvironmentRef *string           `json:"environmentRef,omitempty"`
	TagRefs        []string          `json:"tagRefs,omitempty"`
	ApprovalMode   *string           `json:"approvalMode,omitempty"`
}
