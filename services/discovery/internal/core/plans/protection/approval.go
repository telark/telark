package protection

import (
	"errors"
	"slices"
	"time"
	"unicode/utf8"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	globalshared "github.com/telark/telark/internal/data/shared"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
)

// Sentinels for the non-400 decision outcomes so the handler can map them with errors.Is.
var (
	ErrDecisionNotPending = errors.New(string(ErrApprovalNotPending))
	ErrDecisionStale      = errors.New(string(ErrApprovalStale))
	ErrDecisionSelf       = errors.New(string(ErrApprovalSelfDecision))
)

// The gate must not be one the requester can lower: Production always requires approval, and a
// client-sent mode weaker than the derived one counts only from an Owner.
func ResolveApprovalMode(req *string, environmentRef string, callerOwner bool) string {
	derived := DerivedApprovalMode(environmentRef)
	if derived == plans.ApprovalModeRequired || req == nil {
		return derived
	}
	if *req == plans.ApprovalModeRequired || callerOwner {
		return *req
	}
	return derived
}

func DerivedApprovalMode(environmentRef string) string {
	if environmentRef == dataconstants.CategoryIDEnvProduction {
		return plans.ApprovalModeRequired
	}
	return plans.ApprovalModeAutomatic
}

func RequiresApproval(plan *plans.ProtectionPlan) bool {
	return plan.ApprovalMode == plans.ApprovalModeRequired
}

func EffectiveApprovalMode(mode string) string {
	if mode == constants.EmptyString {
		return plans.ApprovalModeAutomatic
	}
	return mode
}

func NewApprovalRequest(requestedBy, now string, previous *plans.ProtectionPlanApproval) *plans.ProtectionPlanApproval {
	var history []plans.ProtectionPlanApprovalEvent
	if previous != nil {
		history = previous.History
	}
	return &plans.ProtectionPlanApproval{
		State:       plans.ApprovalStatePending,
		RequestedBy: requestedBy,
		RequestedAt: now,
		History:     AppendApprovalEvent(history, plans.ApprovalEventRequested, requestedBy, now, nil),
	}
}

// Drops the oldest entries so the CRD maxItems bound never rejects a patch.
func AppendApprovalEvent(
	history []plans.ProtectionPlanApprovalEvent,
	event, by, at string,
	comment *string,
) []plans.ProtectionPlanApprovalEvent {
	out := make([]plans.ProtectionPlanApprovalEvent, constants.DefaultInitValue, len(history)+constants.DefaultAddValue)
	out = append(out, history...)
	out = append(out, plans.ProtectionPlanApprovalEvent{Event: event, By: by, At: at, Comment: comment})
	if overflow := len(out) - plans.ApprovalHistoryMax; overflow > constants.DefaultInitValue {
		out = out[overflow:]
	}
	return out
}

// Every key is always present (nil when absent) because merge patch merges objects
// key by key and replaces arrays, so a partial value would leave stale decided* behind.
func ApprovalPatchValue(a *plans.ProtectionPlanApproval) map[string]any {
	history := a.History
	if history == nil {
		history = []plans.ProtectionPlanApprovalEvent{}
	}
	return map[string]any{
		FieldApprovalState:       a.State,
		FieldApprovalRequestedBy: a.RequestedBy,
		FieldApprovalRequestedAt: a.RequestedAt,
		FieldApprovalDecidedBy:   nullable(a.DecidedBy),
		FieldApprovalDecidedAt:   nullable(a.DecidedAt),
		FieldApprovalComment:     nullable(a.Comment),
		FieldApprovalHistory:     history,
	}
}

func InitialPhase(plan *plans.ProtectionPlan, now time.Time) {
	nowStr := now.Format(globalshared.DefaultTimeFormat)
	if RequiresApproval(plan) {
		plan.Phase = plans.PhasePendingApproval
		plan.Approval = NewApprovalRequest(plan.CreatedBy, nowStr, nil)
		return
	}
	if plan.TimeMode == plans.TimeModeTimeRange && plan.TimeRange != nil {
		startAt, err := time.Parse(time.RFC3339, plan.TimeRange.StartAt)
		if err == nil && startAt.After(now) {
			plan.Phase = plans.PhaseScheduled
			return
		}
	}
	plan.Phase = plans.PhaseActive
	plan.StartedAt = &nowStr
	plan.StartedBy = &plan.CreatedBy
}

func BuildPendingPatch(plan *plans.ProtectionPlan, requestedBy, now string) map[string]any {
	patch := reactivateShape(plans.PhasePendingApproval, nil, requestedBy, now)
	patch[FieldApproval] = ApprovalPatchValue(NewApprovalRequest(requestedBy, now, plan.Approval))
	return patch
}

func BuildApprovePatch(
	plan *plans.ProtectionPlan,
	targetPhase, approver string,
	comment *string,
	now string,
) map[string]any {
	patch := reactivateShape(targetPhase, plan.RenderedPolicies, approver, now)
	patch[FieldApproval] = decidedApproval(plan.Approval, plans.ApprovalStateApproved, approver, comment, now)
	return patch
}

func BuildRejectPatch(plan *plans.ProtectionPlan, approver, comment, now string) map[string]any {
	patch := BuildCancelPatch(approver, comment, now)
	patch[FieldApproval] = decidedApproval(plan.Approval, plans.ApprovalStateRejected, approver, &comment, now)
	return patch
}

func Activatable(current *plans.ProtectionPlan) bool {
	if current.Phase != plans.PhaseScheduled {
		return false
	}
	if !RequiresApproval(current) {
		return true
	}
	return current.Approval != nil && current.Approval.State == plans.ApprovalStateApproved
}

func ValidateDecision(plan *plans.ProtectionPlan, deciderID string, req planseps.DecideProtectionPlanRequest) error {
	// A malformed request is the caller's mistake whatever state the plan is in.
	if req.Decision != DecisionApproved && req.Decision != DecisionRejected {
		return validation.Invalid(string(ErrApprovalInvalidDecision))
	}
	if req.RequestedAt == constants.EmptyString {
		return validation.Invalid(string(ErrApprovalRequestedAtRequired))
	}
	if err := validateDecisionComment(req); err != nil {
		return err
	}
	if plan.Phase != plans.PhasePendingApproval || plan.Approval == nil {
		return ErrDecisionNotPending
	}
	if req.RequestedAt != plan.Approval.RequestedAt {
		return ErrDecisionStale
	}
	if slices.Contains(ApprovalRequesters(plan.Approval), deciderID) {
		return ErrDecisionSelf
	}
	return nil
}

// Everyone who put the current spec up for approval since the last approval: the creator, a
// reactivator and every material editor, so two users cannot approve each other's changes.
func ApprovalRequesters(a *plans.ProtectionPlanApproval) []string {
	if a == nil {
		return nil
	}
	requesters := []string{a.RequestedBy}
	for i := len(a.History) - constants.DefaultAddValue; i >= constants.DefaultInitValue; i-- {
		event := a.History[i]
		if event.Event == plans.ApprovalEventApproved {
			break
		}
		if event.Event == plans.ApprovalEventRequested {
			requesters = append(requesters, event.By)
		}
	}
	return requesters
}

// Records a material editor of a required plan that is not awaiting a decision (canceled,
// failed, terminated), so a later reactivation by someone else keeps them from approving.
func RecordEditor(previous *plans.ProtectionPlanApproval, editor, now string) *plans.ProtectionPlanApproval {
	approval := *previous
	approval.History = AppendApprovalEvent(previous.History, plans.ApprovalEventRequested, editor, now, nil)
	return &approval
}

func validateDecisionComment(req planseps.DecideProtectionPlanRequest) error {
	if req.Decision == DecisionRejected && (req.Comment == nil || *req.Comment == constants.EmptyString) {
		return validation.Invalid(string(ErrApprovalCommentRequired))
	}
	if req.Comment != nil && utf8.RuneCountInString(*req.Comment) > ApprovalCommentMax {
		return validation.Invalid(string(ErrApprovalCommentTooLong))
	}
	return nil
}

func reactivateShape(phase string, rendered []string, userID, now string) map[string]any {
	if rendered == nil {
		rendered = []string{}
	}
	patch := map[string]any{
		FieldPhase:            phase,
		FieldReason:           nil,
		FieldRenderedPolicies: rendered,
		FieldTerminatedAt:     nil,
		FieldTerminatedBy:     nil,
		FieldLastUpdatedAt:    now,
		FieldLastUpdatedBy:    userID,
		FieldHealth:           plans.HealthUnknown,
		FieldStartedAt:        nil,
		FieldStartedBy:        nil,
	}
	if phase == plans.PhaseActive {
		patch[FieldStartedAt] = now
		patch[FieldStartedBy] = userID
	}
	return patch
}

func decidedApproval(
	previous *plans.ProtectionPlanApproval,
	state, decider string,
	comment *string,
	now string,
) map[string]any {
	approval := plans.ProtectionPlanApproval{}
	if previous != nil {
		approval = *previous
	}
	approval.State = state
	approval.DecidedBy = &decider
	approval.DecidedAt = &now
	approval.Comment = comment
	approval.History = AppendApprovalEvent(approval.History, state, decider, now, comment)
	return ApprovalPatchValue(&approval)
}

func nullable(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
