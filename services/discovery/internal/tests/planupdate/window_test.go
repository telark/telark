package planupdate

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/update"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	zoneUTC = "UTC"
	kindA   = "cat-00004-0001-0001"
	twoAM   = "02:00"
	threeAM = "03:00"
	// editNow is 2026-02-01 00:00 UTC: the daily 23:00 window is open, the later ones are not.
	openWindowEnd   = "2026-02-01T01:00:00Z"
	laterStart      = "2026-02-01T02:00:00Z"
	otherLaterStart = "2026-02-01T04:00:00Z"
	labelWindow     = "window start"
)

func daily(startTime, endTime string) *plans.ProtectionPlanRepeat {
	return &plans.ProtectionPlanRepeat{Days: slices.Clone(plans.RepeatDays), StartTime: startTime, EndTime: endTime, TimeZone: zoneUTC}
}

func openRule() *plans.ProtectionPlanRepeat       { return daily("23:00", "01:00") }
func laterRule() *plans.ProtectionPlanRepeat      { return daily(twoAM, threeAM) }
func otherLaterRule() *plans.ProtectionPlanRepeat { return daily("04:00", "05:00") }

func approved(plan *plans.ProtectionPlan) {
	plan.ApprovalMode = plans.ApprovalModeRequired
	plan.Approval = &plans.ProtectionPlanApproval{
		State: plans.ApprovalStateApproved, RequestedBy: "u-requester", RequestedAt: pastStart, DecidedBy: strptr("u-approver"),
	}
}

// A scheduled series between windows, with nothing deployed.
func scheduledSeries(t *testing.T) *plans.ProtectionPlan {
	t.Helper()
	testutil.ExcludedNamespaces(t)
	return &plans.ProtectionPlan{
		ID:       runPlanID,
		Name:     "guard",
		Severity: plans.SeverityLow,
		Phase:    plans.PhaseScheduled,
		Mode:     plans.ModeAudit,
		TimeMode: plans.TimeModeRepeating,
		Repeat:   laterRule(),
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{runNS}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: tplBlockCreate}},
	}
}

func seriesRequest(plan *plans.ProtectionPlan) *planseps.PrepareProtectionPlanRequest {
	req := requestFor(plan)
	rule := *plan.Repeat
	req.Repeat = &rule
	return req
}

func windowStart(t *testing.T, patch map[string]any) any {
	t.Helper()
	window, ok := patch[protection.FieldWindow].(map[string]any)
	if !ok {
		t.Fatalf("patch window = %v, want an object", patch[protection.FieldWindow])
	}
	return window["startAt"]
}

// A rule counts only while the plan repeats; one left behind by another time mode never makes an
// edit material, as a stale time range does not either.
func TestMaterialChangeRepeat(t *testing.T) {
	cases := []struct {
		name     string
		timeMode string
		stored   *plans.ProtectionPlanRepeat
		edited   *plans.ProtectionPlanRepeat
		want     bool
	}{
		{"same rule", plans.TimeModeRepeating, laterRule(), laterRule(), false},
		{"another time", plans.TimeModeRepeating, laterRule(), otherLaterRule(), true},
		{"another zone", plans.TimeModeRepeating, laterRule(),
			&plans.ProtectionPlanRepeat{Days: slices.Clone(plans.RepeatDays), StartTime: twoAM, EndTime: threeAM, TimeZone: "Europe/Paris"}, true},
		{"fewer days", plans.TimeModeRepeating, laterRule(),
			&plans.ProtectionPlanRepeat{Days: []string{"sun"}, StartTime: twoAM, EndTime: threeAM, TimeZone: zoneUTC}, true},
		{"stale rule of a permanent plan", plans.TimeModePermanent, laterRule(), otherLaterRule(), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := basePlan()
			plan.TimeMode, plan.Repeat = c.timeMode, c.stored
			req := baseRequest(plan)
			req.Repeat = c.edited
			testutil.Equal(t, labelMaterial, update.MaterialChange(plan, req, toPlanPolicies(req.Policies)), c.want)
		})
	}
}

// A time range left behind by a switch to repeating used to count on every later edit: a description
// edit sent an approved series back for approval. Only the current time mode's schedule counts.
func TestStaleScheduleIsNotMaterial(t *testing.T) {
	stale := &plans.ProtectionPlanTimeRange{StartAt: pastStart, EndAt: pastEnd}
	cases := []struct {
		name     string
		timeMode string
		edit     func(*planseps.PrepareProtectionPlanRequest)
		want     bool
	}{
		{"description of a converted series", plans.TimeModeRepeating,
			func(r *planseps.PrepareProtectionPlanRequest) { r.Description = strptr(editedText) }, false},
		{"description of a converted permanent plan", plans.TimeModePermanent,
			func(r *planseps.PrepareProtectionPlanRequest) { r.Description = strptr(editedText) }, false},
		{"a time range plan's new range", plans.TimeModeTimeRange,
			func(r *planseps.PrepareProtectionPlanRequest) { r.TimeRange = window(futureStart, futureEnd) }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := basePlan()
			plan.TimeMode, plan.TimeRange = c.timeMode, stale
			if c.timeMode == plans.TimeModeRepeating {
				plan.Repeat = laterRule()
			}
			req := baseRequest(plan)
			req.Repeat = plan.Repeat
			if c.timeMode == plans.TimeModeTimeRange {
				req.TimeRange = window(pastStart, pastEnd)
			}
			c.edit(req)
			testutil.Equal(t, labelMaterial, update.MaterialChange(plan, req, toPlanPolicies(req.Policies)), c.want)
			patch, _ := update.BuildPatch(plan, req, toPlanPolicies(req.Policies), nil, userID, now)
			testutil.Equal(t, "range patched", patch.TimeRange != nil, c.want)
		})
	}
}

// Time never ends a series: an edited rule only moves the plan between active and scheduled.
func TestTargetPhaseFollowsTheSeries(t *testing.T) {
	cases := []struct {
		name  string
		phase string
		rule  *plans.ProtectionPlanRepeat
		want  string
	}{
		{"scheduled into an open window", plans.PhaseScheduled, openRule(), plans.PhaseActive},
		{"scheduled stays between windows", plans.PhaseScheduled, laterRule(), plans.PhaseScheduled},
		{"active out of its window", plans.PhaseActive, laterRule(), plans.PhaseScheduled},
		{"active stays in its window", plans.PhaseActive, openRule(), plans.PhaseActive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := &plans.ProtectionPlan{Phase: c.phase, TimeMode: plans.TimeModeRepeating, Repeat: laterRule()}
			req := &planseps.PrepareProtectionPlanRequest{TimeMode: plans.TimeModeRepeating, Repeat: c.rule}
			testutil.Equal(t, "phase", update.TargetPhase(plan, req, editNow), c.want)
		})
	}
}

// A kind is metadata, like the environment: it is patched when it changes and "" clears it.
func TestBuildPatchKind(t *testing.T) {
	plan := basePlan()
	plan.KindRef = kindA
	req := baseRequest(plan)
	testutil.Equal(t, "unchanged kind", update.MaterialChange(plan, req, plan.Policies), false)
	req.KindRef = strptr(constants.EmptyString)

	patch, changed := update.BuildPatch(plan, req, plan.Policies, nil, userID, now)
	testutil.Equal(t, nameChanged, changed, true)
	if patch.KindRef == nil || *patch.KindRef != constants.EmptyString {
		t.Fatalf("kindRef = %v, want a clear", patch.KindRef)
	}
	testutil.Equal(t, labelMaterial, update.MaterialChange(plan, req, plan.Policies), false)
}

// An approved series keeps its approved spec while a window is open; the refusal says when the
// window ends, and nothing in the cluster moves.
func TestRunRefusesMaterialEditsInAnOpenApprovedWindow(t *testing.T) {
	plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
	plan.TimeMode, plan.Repeat = plans.TimeModeRepeating, openRule()
	approved(plan)
	req := seriesRequest(plan)
	req.Policies = append(req.Policies, planseps.PolicyRequest{TemplateID: tplBlockDelete})
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	dyn := clusterWith(t, live...)

	_, err := update.Run(context.Background(), runDeps(dyn, &reportsRecorder{}), userID, plan.ID, req)
	testutil.Equal(t, "validation error", validation.IsValidation(err), true)
	testutil.Equal(t, "names the window end", err != nil && strings.Contains(err.Error(), openWindowEnd), true)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultInitValue)
	testutil.Equal(t, "cluster calls", len(dyn.Actions()), constants.DefaultInitValue)
}

// Kind, name, description and the like are never material, so they stay editable mid-window.
func TestRunAppliesNonMaterialEditsInAnOpenApprovedWindow(t *testing.T) {
	plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
	plan.TimeMode, plan.Repeat = plans.TimeModeRepeating, openRule()
	approved(plan)
	req := seriesRequest(plan)
	req.Description = strptr(editedText)
	req.KindRef = strptr(kindA)
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)

	_, err := update.Run(context.Background(), runDeps(clusterWith(t, live...), &reportsRecorder{}), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultAddValue)
	_, phaseSet := stub.patches[constants.DefaultInitValue][protection.FieldPhase]
	testutil.Equal(t, "phase untouched", phaseSet, false)
	testutil.Equal(t, "kind patched", stub.patches[constants.DefaultInitValue]["kindRef"], any(kindA))
}

// One approval covers the series, and a material edit between windows asks for a new one: the
// editor becomes the requester, approvers are told, and no window opens meanwhile.
func TestRunAsksForApprovalAgainBetweenWindows(t *testing.T) {
	plan := scheduledSeries(t)
	approved(plan)
	req := seriesRequest(plan)
	req.Policies = append(req.Policies, planseps.PolicyRequest{TemplateID: tplBlockDelete})
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	deps := runDeps(clusterWith(t), &reportsRecorder{})
	activated, notified := false, false
	deps.Activate = func(context.Context, *plans.ProtectionPlan) error { activated = true; return nil }
	deps.NotifyApprovers = func(*plans.ProtectionPlan) { notified = true }

	_, err := update.Run(context.Background(), deps, userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultAddValue)
	patch := stub.patches[constants.DefaultInitValue]
	testutil.Equal(t, "phase", patch[protection.FieldPhase], any(plans.PhasePendingApproval))
	approval, ok := patch[protection.FieldApproval].(map[string]any)
	if !ok {
		t.Fatalf("approval = %v, want an object", patch[protection.FieldApproval])
	}
	testutil.Equal(t, "approval state", approval[protection.FieldApprovalState], any(plans.ApprovalStatePending))
	testutil.Equal(t, "requested by the editor", approval[protection.FieldApprovalRequestedBy], any(userID))
	testutil.Equal(t, "approvers notified", notified, true)
	testutil.Equal(t, "no window opened", activated, false)
}

// An edit that moves an open window away closes it like the controller would: the run is
// checkpointed while its policies are still live, then gets its own window report.
func TestRunParkClosesTheWindowWithItsReport(t *testing.T) {
	plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
	plan.TimeMode, plan.Repeat = plans.TimeModeRepeating, openRule()
	req := seriesRequest(plan)
	req.Repeat = laterRule()
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	dyn := clusterWith(t, live...)
	rec := &reportsRecorder{}

	_, err := update.Run(context.Background(), runDeps(dyn, rec), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "checkpoints", len(rec.checkpoints), constants.DefaultAddValue)
	testutil.Equal(t, "policies live at checkpoint", slices.Equal(rec.liveAtCheck, plan.RenderedPolicies), true)
	testutil.Equal(t, "policies withdrawn", len(liveNames(context.Background(), dyn)), constants.DefaultInitValue)
	testutil.Equal(t, "parked", stub.patches[constants.DefaultInitValue][protection.FieldPhase], any(plans.PhaseScheduled))
	testutil.Equal(t, labelWindow, windowStart(t, stub.patches[constants.DefaultInitValue]), any(laterStart))
	testutil.Equal(t, "captures", len(rec.captures), constants.DefaultAddValue)
	testutil.Equal(t, "trigger", rec.captures[constants.DefaultInitValue].trigger, reportseps.TriggerWindow)
	testutil.Equal(t, "reason", rec.captures[constants.DefaultInitValue].reason, protection.ReasonWindowClosedByEdit)
}

// End to end: a series converted from a time range keeps its approval through a description edit.
func TestRunKeepsTheApprovalOfAConvertedSeries(t *testing.T) {
	plan := scheduledSeries(t)
	approved(plan)
	plan.TimeRange = &plans.ProtectionPlanTimeRange{StartAt: pastStart, EndAt: pastEnd}
	req := seriesRequest(plan)
	req.Description = strptr(editedText)
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)

	_, err := update.Run(context.Background(), runDeps(clusterWith(t), &reportsRecorder{}), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultAddValue)
	_, phaseSet := stub.patches[constants.DefaultInitValue][protection.FieldPhase]
	testutil.Equal(t, "approval kept", phaseSet, false)
}

// The plan the edit returns already shows the window its new rule puts it in.
func TestRunStampsTheEditedRulesWindow(t *testing.T) {
	plan := scheduledSeries(t)
	req := seriesRequest(plan)
	req.Repeat = otherLaterRule()
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)

	_, err := update.Run(context.Background(), runDeps(clusterWith(t), &reportsRecorder{}), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultAddValue)
	testutil.Equal(t, labelWindow, windowStart(t, stub.patches[constants.DefaultInitValue]), any(otherLaterStart))
	_, ruleSet := stub.patches[constants.DefaultInitValue]["repeat"]
	testutil.Equal(t, "rule patched", ruleSet, true)
}
