package planupdate

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	labelMaterial = "material"
	labelChanged  = "approvalMode changed"
	otherNS       = "other-ns"
	priorityHigh  = 9
	kindConfigMap = "ConfigMap"
	kindSecret    = "Secret"
)

type materialCase struct {
	name   string
	stored *plans.ProtectionPlanScopeExclusions
	mutate func(req *planseps.PrepareProtectionPlanRequest)
	want   bool
}

func storedKinds() *plans.ProtectionPlanScopeExclusions {
	return &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap, kindSecret}}
}

func materialCases() []materialCase {
	return []materialCase{
		{name: "unchanged", mutate: func(*planseps.PrepareProtectionPlanRequest) {}, want: false},
		{name: "policies", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Policies = append(r.Policies, planseps.PolicyRequest{TemplateID: "block-delete"})
		}, want: true},
		{name: "targets", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Namespaces = []string{otherNS}
		}, want: true},
		{name: "mode", mutate: func(r *planseps.PrepareProtectionPlanRequest) { r.Mode = plans.ModeEnforce }, want: true},
		{name: "timeMode", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.TimeMode = plans.TimeModeTimeRange
		}, want: true},
		{name: "timeRange", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.TimeRange = &planseps.TimeRangeRequest{StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z"}
		}, want: true},
		{name: "name", mutate: func(r *planseps.PrepareProtectionPlanRequest) { r.Name = nameChanged }, want: false},
		{name: "description", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Description = strptr(nameChanged)
		}, want: false},
		{name: "severity", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Severity = plans.SeverityHigh
		}, want: false},
		{name: "priority", mutate: func(r *planseps.PrepareProtectionPlanRequest) { r.Priority = priorityHigh }, want: false},
		{name: "participants", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.ParticipantRefs = []string{userID}
		}, want: false},
		{name: "environment", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.EnvironmentRef = strptr(envA)
		}, want: false},
		{name: "tags", mutate: func(r *planseps.PrepareProtectionPlanRequest) { r.TagRefs = []string{tagA} }, want: false},
		{name: "exclusions kinds", mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap}}
		}, want: true},
		{name: "exclusions cleared", stored: storedKinds(), mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{}
		}, want: true},
		{name: "exclusions untouched", stored: storedKinds(), mutate: func(*planseps.PrepareProtectionPlanRequest) {}, want: false},
		{name: "exclusions reordered", stored: storedKinds(), mutate: func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindSecret, kindConfigMap}}
		}, want: false},
	}
}

func toPlanPolicies(reqs []planseps.PolicyRequest) []plans.ProtectionPlanPolicy {
	out := make([]plans.ProtectionPlanPolicy, len(reqs))
	for i, r := range reqs {
		out[i] = plans.ProtectionPlanPolicy{TemplateID: r.TemplateID}
	}
	return out
}

func TestMaterialChangeMatrix(t *testing.T) {
	for _, tc := range materialCases() {
		t.Run(tc.name, func(t *testing.T) {
			plan := basePlan()
			plan.Scope.Exclusions = tc.stored
			req := baseRequest(plan)
			tc.mutate(req)
			testutil.Equal(t, labelMaterial, update.MaterialChange(plan, req, toPlanPolicies(req.Policies)), tc.want)
		})
	}
}

func TestApprovalModeImmutableComparesEffectiveModes(t *testing.T) {
	cases := []struct {
		name   string
		stored string
		req    *string
		want   bool
	}{
		{name: "absent request", stored: plans.ApprovalModeRequired, req: nil, want: false},
		{name: "legacy + automatic", stored: "", req: strptr(plans.ApprovalModeAutomatic), want: false},
		{name: "legacy + required", stored: "", req: strptr(plans.ApprovalModeRequired), want: true},
		{name: "required + required", stored: plans.ApprovalModeRequired, req: strptr(plans.ApprovalModeRequired), want: false},
		{name: "required + automatic", stored: plans.ApprovalModeRequired, req: strptr(plans.ApprovalModeAutomatic), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := basePlan()
			plan.ApprovalMode = tc.stored
			req := baseRequest(plan)
			req.ApprovalMode = tc.req
			testutil.Equal(t, labelChanged, update.ApprovalModeChanged(plan, req), tc.want)
		})
	}
}
