package planapproval

import (
	stderrors "errors"
	"strings"
	"testing"
	"time"

	dataconstants "github.com/telark/data/constants"
	"github.com/telark/data/errors"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	stagingEnvironmentID = "cat-00002-0001-0002"
	requester            = "user-req"
	approver             = "user-app"
	requestedAt          = "2026-01-01T00:00:00Z"
	decidedAt            = "2026-01-02T00:00:00Z"
	commentText          = "looks fine"
	approvalPatchKeys    = 7
	overflowHistoryLen   = plans.ApprovalHistoryMax + 1
	pastStart            = "2020-01-01T00:00:00Z"
	futureStart          = "2999-01-01T00:00:00Z"
	futureEnd            = "2999-01-02T00:00:00Z"
	singleEvent          = 1
	labelState           = "state"
	labelLastEvent       = "last event"
	labelDecidedBy       = "decidedBy"
	labelHistoryLen      = "history len"
)

var testNow = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)

func strptr(s string) *string { return &s }

func pendingPlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:           "plan-1",
		Phase:        plans.PhasePendingApproval,
		ApprovalMode: plans.ApprovalModeRequired,
		Approval: &plans.ProtectionPlanApproval{
			State:       plans.ApprovalStatePending,
			RequestedBy: requester,
			RequestedAt: requestedAt,
			History: []plans.ProtectionPlanApprovalEvent{
				{Event: plans.ApprovalEventRequested, By: requester, At: requestedAt},
			},
		},
	}
}

func approvalOf(t *testing.T, patch map[string]any) map[string]any {
	t.Helper()
	approval, ok := patch[protection.FieldApproval].(map[string]any)
	if !ok {
		t.Fatalf("approval = %v, want a map", patch[protection.FieldApproval])
	}
	return approval
}

func historyOf(t *testing.T, approval map[string]any) []plans.ProtectionPlanApprovalEvent {
	t.Helper()
	history, ok := approval[protection.FieldApprovalHistory].([]plans.ProtectionPlanApprovalEvent)
	if !ok {
		t.Fatalf("history = %v, want a typed slice", approval[protection.FieldApprovalHistory])
	}
	return history
}

func requireNil(t *testing.T, patch map[string]any, key string) {
	t.Helper()
	value, present := patch[key]
	if !present {
		t.Fatalf("%s missing from patch", key)
	}
	if value != nil {
		t.Fatalf("%s = %v, want nil", key, value)
	}
}

func TestResolveApprovalMode(t *testing.T) {
	cases := []struct {
		name string
		req  *string
		env  string
		want string
	}{
		{
			"explicit automatic on production wins",
			strptr(plans.ApprovalModeAutomatic),
			dataconstants.CategoryIDEnvProduction,
			plans.ApprovalModeAutomatic,
		},
		{"explicit required on staging wins", strptr(plans.ApprovalModeRequired), stagingEnvironmentID, plans.ApprovalModeRequired},
		{"nil on production derives required", nil, dataconstants.CategoryIDEnvProduction, plans.ApprovalModeRequired},
		{"nil on staging derives automatic", nil, stagingEnvironmentID, plans.ApprovalModeAutomatic},
		{"nil without environment derives automatic", nil, constants.EmptyString, plans.ApprovalModeAutomatic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "mode", protection.ResolveApprovalMode(c.req, c.env), c.want)
		})
	}
	testutil.Equal(t, "effective empty", protection.EffectiveApprovalMode(constants.EmptyString), plans.ApprovalModeAutomatic)
	testutil.Equal(t, "effective required", protection.EffectiveApprovalMode(plans.ApprovalModeRequired), plans.ApprovalModeRequired)
	testutil.Equal(t, "requires legacy", protection.RequiresApproval(&plans.ProtectionPlan{}), false)
	testutil.Equal(t, "requires required", protection.RequiresApproval(&plans.ProtectionPlan{ApprovalMode: plans.ApprovalModeRequired}), true)
}

func TestInitialPhaseParksRequiredPlans(t *testing.T) {
	now := testNow

	required := &plans.ProtectionPlan{CreatedBy: requester, ApprovalMode: plans.ApprovalModeRequired}
	protection.InitialPhase(required, now)
	testutil.Equal(t, "required phase", required.Phase, plans.PhasePendingApproval)
	if required.Approval == nil {
		t.Fatal("required plan must carry an approval request")
	}
	testutil.Equal(t, labelState, required.Approval.State, plans.ApprovalStatePending)
	testutil.Equal(t, "requestedBy", required.Approval.RequestedBy, requester)
	testutil.Equal(t, labelHistoryLen, len(required.Approval.History), singleEvent)
	if required.StartedAt != nil || required.StartedBy != nil {
		t.Fatal("pending plan must not be started")
	}

	automatic := &plans.ProtectionPlan{CreatedBy: requester}
	protection.InitialPhase(automatic, now)
	testutil.Equal(t, "automatic phase", automatic.Phase, plans.PhaseActive)
	if automatic.StartedAt == nil || automatic.StartedBy == nil || *automatic.StartedBy != requester {
		t.Fatal("active plan must be started by its creator")
	}
	if automatic.Approval != nil {
		t.Fatal("automatic plan must not carry an approval request")
	}

	scheduled := &plans.ProtectionPlan{
		CreatedBy: requester,
		TimeMode:  plans.TimeModeTimeRange,
		TimeRange: &plans.ProtectionPlanTimeRange{StartAt: futureStart, EndAt: futureEnd},
	}
	protection.InitialPhase(scheduled, now)
	testutil.Equal(t, "scheduled phase", scheduled.Phase, plans.PhaseScheduled)
}

func TestBuildApprovePatchActiveAndScheduled(t *testing.T) {
	plan := pendingPlan()
	plan.RenderedPolicies = []string{"pol-1"}

	active := protection.BuildApprovePatch(plan, plans.PhaseActive, approver, strptr(commentText), decidedAt)
	testutil.Equal(t, "active phase", active[protection.FieldPhase], any(plans.PhaseActive))
	testutil.Equal(t, "startedAt", active[protection.FieldStartedAt], any(decidedAt))
	testutil.Equal(t, "startedBy", active[protection.FieldStartedBy], any(approver))
	testutil.Equal(t, "lastUpdatedBy", active[protection.FieldLastUpdatedBy], any(approver))
	testutil.Equal(t, "health", active[protection.FieldHealth], any(plans.HealthUnknown))
	requireNil(t, active, protection.FieldReason)
	requireNil(t, active, protection.FieldTerminatedAt)
	requireNil(t, active, protection.FieldTerminatedBy)
	rendered, ok := active[protection.FieldRenderedPolicies].([]string)
	testutil.Equal(t, "rendered typed", ok, true)
	testutil.Equal(t, "rendered len", len(rendered), singleEvent)
	approval := approvalOf(t, active)
	testutil.Equal(t, labelState, approval[protection.FieldApprovalState], any(plans.ApprovalStateApproved))
	testutil.Equal(t, labelDecidedBy, approval[protection.FieldApprovalDecidedBy], any(approver))
	testutil.Equal(t, "decidedAt", approval[protection.FieldApprovalDecidedAt], any(decidedAt))
	testutil.Equal(t, "comment", approval[protection.FieldApprovalComment], any(commentText))
	testutil.Equal(t, "requestedBy kept", approval[protection.FieldApprovalRequestedBy], any(requester))
	history := historyOf(t, approval)
	testutil.Equal(t, labelHistoryLen, len(history), len(plan.Approval.History)+constants.DefaultAddValue)
	testutil.Equal(t, labelLastEvent, history[len(history)-constants.DefaultAddValue].Event, plans.ApprovalEventApproved)

	scheduled := protection.BuildApprovePatch(plan, plans.PhaseScheduled, approver, nil, decidedAt)
	testutil.Equal(t, "scheduled phase", scheduled[protection.FieldPhase], any(plans.PhaseScheduled))
	requireNil(t, scheduled, protection.FieldStartedAt)
	requireNil(t, scheduled, protection.FieldStartedBy)
	requireNil(t, approvalOf(t, scheduled), protection.FieldApprovalComment)
}

func TestBuildRejectPatch(t *testing.T) {
	plan := pendingPlan()
	patch := protection.BuildRejectPatch(plan, approver, commentText, decidedAt)
	testutil.Equal(t, "phase", patch[protection.FieldPhase], any(plans.PhaseCanceled))
	testutil.Equal(t, "reason", patch[protection.FieldReason], any(commentText))
	testutil.Equal(t, "terminatedBy", patch[protection.FieldTerminatedBy], any(approver))
	testutil.Equal(t, "terminatedAt", patch[protection.FieldTerminatedAt], any(decidedAt))
	approval := approvalOf(t, patch)
	testutil.Equal(t, labelState, approval[protection.FieldApprovalState], any(plans.ApprovalStateRejected))
	testutil.Equal(t, labelDecidedBy, approval[protection.FieldApprovalDecidedBy], any(approver))
	testutil.Equal(t, "comment", approval[protection.FieldApprovalComment], any(commentText))
	history := historyOf(t, approval)
	testutil.Equal(t, labelLastEvent, history[len(history)-constants.DefaultAddValue].Event, plans.ApprovalEventRejected)
}

func TestBuildPendingPatchClearsDecision(t *testing.T) {
	plan := pendingPlan()
	plan.Phase = plans.PhaseCanceled
	plan.Approval.State = plans.ApprovalStateRejected
	plan.Approval.DecidedBy = strptr(approver)
	plan.Approval.DecidedAt = strptr(decidedAt)
	plan.Approval.Comment = strptr(commentText)

	patch := protection.BuildPendingPatch(plan, approver, decidedAt)
	testutil.Equal(t, "phase", patch[protection.FieldPhase], any(plans.PhasePendingApproval))
	testutil.Equal(t, "health", patch[protection.FieldHealth], any(plans.HealthUnknown))
	for _, key := range []string{
		protection.FieldReason,
		protection.FieldTerminatedAt,
		protection.FieldTerminatedBy,
		protection.FieldStartedAt,
		protection.FieldStartedBy,
	} {
		requireNil(t, patch, key)
	}
	rendered, ok := patch[protection.FieldRenderedPolicies].([]string)
	testutil.Equal(t, "rendered typed", ok, true)
	testutil.Equal(t, "rendered empty", len(rendered), constants.DefaultInitValue)

	approval := approvalOf(t, patch)
	testutil.Equal(t, labelState, approval[protection.FieldApprovalState], any(plans.ApprovalStatePending))
	testutil.Equal(t, "requestedBy", approval[protection.FieldApprovalRequestedBy], any(approver))
	testutil.Equal(t, "requestedAt", approval[protection.FieldApprovalRequestedAt], any(decidedAt))
	requireNil(t, approval, protection.FieldApprovalDecidedBy)
	requireNil(t, approval, protection.FieldApprovalDecidedAt)
	requireNil(t, approval, protection.FieldApprovalComment)
	history := historyOf(t, approval)
	testutil.Equal(t, "history kept + requested", len(history), len(plan.Approval.History)+constants.DefaultAddValue)
	testutil.Equal(t, labelLastEvent, history[len(history)-constants.DefaultAddValue].Event, plans.ApprovalEventRequested)
}

func TestApprovalPatchValueAlwaysSevenKeys(t *testing.T) {
	bare := protection.ApprovalPatchValue(&plans.ProtectionPlanApproval{
		State:       plans.ApprovalStatePending,
		RequestedBy: requester,
		RequestedAt: requestedAt,
	})
	testutil.Equal(t, "bare keys", len(bare), approvalPatchKeys)
	requireNil(t, bare, protection.FieldApprovalDecidedBy)
	requireNil(t, bare, protection.FieldApprovalDecidedAt)
	requireNil(t, bare, protection.FieldApprovalComment)
	testutil.Equal(t, "bare history len", len(historyOf(t, bare)), constants.DefaultInitValue)

	full := pendingPlan().Approval
	full.DecidedBy = strptr(approver)
	full.DecidedAt = strptr(decidedAt)
	full.Comment = strptr(commentText)
	value := protection.ApprovalPatchValue(full)
	testutil.Equal(t, "full keys", len(value), approvalPatchKeys)
	testutil.Equal(t, labelDecidedBy, value[protection.FieldApprovalDecidedBy], any(approver))
	testutil.Equal(t, labelHistoryLen, len(historyOf(t, value)), len(full.History))
}

func TestAppendApprovalEventCaps(t *testing.T) {
	var history []plans.ProtectionPlanApprovalEvent
	for i := range overflowHistoryLen {
		at := time.Unix(int64(i), 0).UTC().Format(time.RFC3339)
		history = protection.AppendApprovalEvent(history, plans.ApprovalEventRequested, requester, at, nil)
	}
	testutil.Equal(t, "capped len", len(history), plans.ApprovalHistoryMax)
	testutil.Equal(t, "oldest dropped", history[constants.DefaultInitValue].At, time.Unix(1, 0).UTC().Format(time.RFC3339))
	newest := history[len(history)-constants.DefaultAddValue]
	testutil.Equal(t, "newest kept", newest.At, time.Unix(plans.ApprovalHistoryMax, 0).UTC().Format(time.RFC3339))

	withComment := protection.AppendApprovalEvent(
		nil, plans.ApprovalEventRejected, approver, decidedAt, strptr(commentText))
	testutil.Equal(t, "comment kept", *withComment[constants.DefaultInitValue].Comment, commentText)
}

func planIn(phase, mode string, approval *plans.ProtectionPlanApproval) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{Phase: phase, ApprovalMode: mode, Approval: approval}
}

func TestActivatable(t *testing.T) {
	approved := &plans.ProtectionPlanApproval{State: plans.ApprovalStateApproved}
	pending := &plans.ProtectionPlanApproval{State: plans.ApprovalStatePending}
	cases := []struct {
		name string
		plan *plans.ProtectionPlan
		want bool
	}{
		{"legacy scheduled", planIn(plans.PhaseScheduled, constants.EmptyString, nil), true},
		{"automatic scheduled", planIn(plans.PhaseScheduled, plans.ApprovalModeAutomatic, nil), true},
		{"required approved scheduled", planIn(plans.PhaseScheduled, plans.ApprovalModeRequired, approved), true},
		{"required pending scheduled", planIn(plans.PhaseScheduled, plans.ApprovalModeRequired, pending), false},
		{"required no approval scheduled", planIn(plans.PhaseScheduled, plans.ApprovalModeRequired, nil), false},
		{"pending phase", planIn(plans.PhasePendingApproval, plans.ApprovalModeRequired, approved), false},
		{"canceled phase", planIn(plans.PhaseCanceled, constants.EmptyString, nil), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "activatable", protection.Activatable(c.plan), c.want)
		})
	}
}

func decide(decision, at string, comment *string) planseps.DecideProtectionPlanRequest {
	return planseps.DecideProtectionPlanRequest{Decision: decision, RequestedAt: at, Comment: comment}
}

func invalid(msg errors.Error) error { return validation.Invalid(string(msg)) }

type decisionCase struct {
	name       string
	plan       *plans.ProtectionPlan
	decider    string
	req        planseps.DecideProtectionPlanRequest
	wantErr    error
	validation bool
}

func decisionCases() []decisionCase {
	notPending := pendingPlan()
	notPending.Phase = plans.PhaseActive
	noApproval := pendingPlan()
	noApproval.Approval = nil
	approve := decide(protection.DecisionApproved, requestedAt, nil)
	reject := decide(protection.DecisionRejected, requestedAt, strptr(commentText))
	longComment := strings.Repeat("x", protection.ApprovalCommentMax+constants.DefaultAddValue)

	return []decisionCase{
		{"approve ok", pendingPlan(), approver, approve, nil, false},
		{"reject ok", pendingPlan(), approver, reject, nil, false},
		{"not pending phase", notPending, approver, approve, protection.ErrDecisionNotPending, false},
		{"nil approval", noApproval, approver, approve, protection.ErrDecisionNotPending, false},
		{
			"requestedAt missing", pendingPlan(), approver,
			decide(protection.DecisionApproved, constants.EmptyString, nil),
			invalid(protection.ErrApprovalRequestedAtRequired), true,
		},
		{
			"stale fingerprint", pendingPlan(), approver,
			decide(protection.DecisionApproved, decidedAt, nil),
			protection.ErrDecisionStale, false,
		},
		{"self decision", pendingPlan(), requester, approve, protection.ErrDecisionSelf, false},
		{
			"invalid decision", pendingPlan(), approver,
			decide("maybe", requestedAt, nil),
			invalid(protection.ErrApprovalInvalidDecision), true,
		},
		{
			"reject without comment", pendingPlan(), approver,
			decide(protection.DecisionRejected, requestedAt, nil),
			invalid(protection.ErrApprovalCommentRequired), true,
		},
		{
			"reject empty comment", pendingPlan(), approver,
			decide(protection.DecisionRejected, requestedAt, strptr(constants.EmptyString)),
			invalid(protection.ErrApprovalCommentRequired), true,
		},
		{
			"comment too long", pendingPlan(), approver,
			decide(protection.DecisionApproved, requestedAt, &longComment),
			invalid(protection.ErrApprovalCommentTooLong), true,
		},
	}
}

func TestValidateDecision(t *testing.T) {
	for _, c := range decisionCases() {
		t.Run(c.name, func(t *testing.T) {
			err := protection.ValidateDecision(c.plan, c.decider, c.req)
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want %v, got nil", c.wantErr)
			}
			testutil.Equal(t, "validation error", validation.IsValidation(err), c.validation)
			if c.validation {
				testutil.Equal(t, "message", err.Error(), c.wantErr.Error())
				return
			}
			testutil.Equal(t, "typed error", stderrors.Is(err, c.wantErr), true)
		})
	}
}
