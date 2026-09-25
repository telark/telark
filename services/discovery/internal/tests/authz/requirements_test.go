package authz

import (
	"context"
	"strings"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/routes"
	"github.com/telark/rest/base"
	analyzeps "github.com/telark/rest/endpoints/analyze"
	insightseps "github.com/telark/rest/endpoints/insights"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
)

func TestRequirementsCoverEveryRoute(t *testing.T) {
	requirements := authz.Requirements()

	var missing []string
	for _, route := range routes.Routes {
		key := route.Method + " " + route.Pattern
		if _, found := requirements[key]; !found {
			missing = append(missing, key)
		}
	}

	if len(missing) > constants.DefaultInitValue {
		t.Errorf("routes with no authz requirement (%d):", len(missing))
		for _, key := range missing {
			t.Errorf("  %s", key)
		}
	}
}

func TestNoRequirementWithoutRoute(t *testing.T) {
	registered := map[string]bool{}
	for _, route := range routes.Routes {
		registered[route.Method+" "+route.Pattern] = true
	}

	for key := range authz.Requirements() {
		if !registered[key] {
			t.Errorf("requirement %q matches no registered route", key)
		}
	}
}

// This service can write workloads across every namespace, so an unauthenticated
// route here is worse than one on the exporter. Only probes qualify.
func TestOnlyProbesArePublic(t *testing.T) {
	allowed := map[string]bool{
		"GET /api/v1/status/live":  true,
		"GET /api/v1/status/ready": true,
	}

	for key, requirement := range authz.Requirements() {
		if requirement.Access == xauthz.AccessPublic && !allowed[key] {
			t.Errorf("route %q is public but is not a probe", key)
		}
	}
}

// Nothing here acts on the caller's own record, so nothing here may skip the
// scope check. The namespaces handler checks its two scopes itself (NamespacesAllowed).
func TestNoRouteSkipsScopeCheck(t *testing.T) {
	handlerChecked := router.Key(base.Get, analyzeps.GetAllNamespaces)
	if got := authz.Requirements()[handlerChecked]; got != xauthz.Authenticated {
		t.Errorf("route %q requirement = %+v, want session-only so an insights reader reaches the handler", handlerChecked, got)
	}
	for key, requirement := range authz.Requirements() {
		if requirement.Access == xauthz.AccessAuthenticated && key != handlerChecked {
			t.Errorf("route %q requires only a session and no scope", key)
		}
	}
}

func TestInsightsRoutesRequireInsightsRead(t *testing.T) {
	requirements := authz.Requirements()
	for _, endpoint := range []base.Endpoint{insightseps.Applications, insightseps.List} {
		key := router.Key(base.Get, endpoint)
		want := xauthz.Read(roledata.ScopeInsights)
		if got := requirements[key]; got != want {
			t.Errorf("route %q requirement = %+v, want %+v", key, got, want)
		}
	}
}

func TestNamespacesAllowed(t *testing.T) {
	levels := func(scope string, level roledata.PermissionLevel) *xauthz.Identity {
		return &xauthz.Identity{Grants: xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: level}}}
	}

	cases := []struct {
		name     string
		identity *xauthz.Identity
		want     bool
	}{
		{name: "applications reader", identity: levels(roledata.ScopeApplications, roledata.PermissionLevelReadOnly), want: true},
		{name: "insights reader", identity: levels(roledata.ScopeInsights, roledata.PermissionLevelReadOnly), want: true},
		{name: "admin via ALL", identity: levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), want: true},
		{name: "internal peer", identity: &xauthz.Identity{Internal: true}, want: true},
		{name: "settings owner only", identity: levels(roledata.ScopeSettings, roledata.PermissionLevelOwner), want: false},
		{name: "unknown level", identity: levels(roledata.ScopeInsights, roledata.PermissionLevel("Viewer")), want: false},
		{name: "no identity", want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.identity != nil {
				ctx = xauthz.WithIdentity(ctx, *c.identity)
			}
			if got := authz.NamespacesAllowed(ctx); got != c.want {
				t.Errorf("allowed = %v, want %v", got, c.want)
			}
		})
	}
}

func TestScopedRequirementsAreComplete(t *testing.T) {
	for key, requirement := range authz.Requirements() {
		if requirement.Access != xauthz.AccessScoped {
			continue
		}
		if requirement.Scope == constants.EmptyString {
			t.Errorf("route %q is scoped but declares no scope", key)
		}
		if requirement.MinLevel.Rank() == constants.DefaultInitValue {
			t.Errorf("route %q declares an unusable level %q", key, requirement.MinLevel)
		}
	}
}

// The routes that mutate live cluster state must never be readable-level.
func TestDestructiveRoutesRequireWriteAccess(t *testing.T) {
	destructive := []string{
		"POST /api/v1/resources/applications/{name}/rollbacks/trigger",
		"POST /api/v1/resources/applications/{name}/rollbacks/{rollbackId}/abort",
		"POST /api/v1/resources/applications/{name}/sync",
		"POST /api/v1/resources/applications/{name}/reset",
	}

	requirements := authz.Requirements()
	for _, key := range destructive {
		requirement, found := requirements[key]
		if !found {
			t.Errorf("destructive route %q has no requirement", key)
			continue
		}
		if requirement.MinLevel.Rank() < constants.TwoValue {
			t.Errorf("destructive route %q only needs %q", key, requirement.MinLevel)
		}
	}
}

// Deciding on a plan deploys or discards it on someone else's behalf, so it is
// Owner-only. Approve and reject share the route, so each decision's deny rule
// is applied in the handler rather than on the route.
func TestDecideRouteIsOwnerWithPerDecisionRules(t *testing.T) {
	requirement, found := authz.Requirements()[router.Key(base.Post, planseps.DecideProtectionPlan)]
	if !found {
		t.Fatal("decide route has no requirement")
	}
	if requirement.Scope != roledata.ScopeProtectionPlans {
		t.Errorf("scope = %q, want %q", requirement.Scope, roledata.ScopeProtectionPlans)
	}
	if requirement.MinLevel != roledata.PermissionLevelOwner {
		t.Errorf("min level = %q, want %q", requirement.MinLevel, roledata.PermissionLevelOwner)
	}
	if requirement.Rule != constants.EmptyString {
		t.Errorf("rule = %q, want none", requirement.Rule)
	}

	decisions := map[string]struct {
		requirement xauthz.Requirement
		action      string
	}{
		"approve": {authz.ApprovePlanRequirement(), roledata.ActionApproveProtectionPlan},
		"reject":  {authz.RejectPlanRequirement(), roledata.ActionRejectProtectionPlan},
	}
	for name, decision := range decisions {
		if decision.requirement.MinLevel != roledata.PermissionLevelOwner {
			t.Errorf("%s min level = %q, want %q", name, decision.requirement.MinLevel, roledata.PermissionLevelOwner)
		}
		want := xauthz.RuleKey(roledata.ScopeProtectionPlans, decision.action)
		if decision.requirement.Rule != want {
			t.Errorf("%s rule = %q, want %q", name, decision.requirement.Rule, want)
		}
	}
}

// The dashboard's role editor offers these exact strings, so a rename on either
// side must fail here.
func TestPlanRuleKeysMatchDashboardVocabulary(t *testing.T) {
	want := map[string]string{
		"GET /api/v1/plans/protection/templates":              "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/plans/protection/{id}/status":            "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/plans/protection/{id}/violations":        "protection-plans.viewprotectionplanviolations.deny",
		"POST /api/v1/plans/protection/prepare":               "protection-plans.createprotectionplan.deny",
		"POST /api/v1/plans/protection/{id}/update":           "protection-plans.editprotectionplan.deny",
		"POST /api/v1/plans/protection/{id}/duplicate":        "protection-plans.duplicateprotectionplan.deny",
		"POST /api/v1/plans/protection/{id}/reactivate":       "protection-plans.reactivateprotectionplan.deny",
		"POST /api/v1/plans/protection/{id}/cancel":           "protection-plans.cancelprotectionplan.deny",
		"POST /api/v1/plans/protection/{id}/reports/generate": "protection-plans.generateprotectionplanreport.deny",
		"DELETE /api/v1/plans/protection/{id}/clear":          "protection-plans.deleteprotectionplan.deny",
	}

	requirements := authz.Requirements()
	for key, rule := range want {
		requirement, found := requirements[key]
		if !found {
			t.Errorf("route %q has no requirement", key)
			continue
		}
		if requirement.Rule != rule {
			t.Errorf("route %q rule = %q, want %q", key, requirement.Rule, rule)
		}
	}
}

// A plan route without a rule is an action no role can withhold; decide
// applies its rule per decision in the handler.
func TestEveryPlanRouteIsDenyable(t *testing.T) {
	decideKey := router.Key(base.Post, planseps.DecideProtectionPlan)
	prefix := roledata.ScopeProtectionPlans + "."

	for key, requirement := range authz.Requirements() {
		if requirement.Scope != roledata.ScopeProtectionPlans || key == decideKey {
			continue
		}
		if !strings.HasPrefix(requirement.Rule, prefix) || !strings.HasSuffix(requirement.Rule, ".deny") {
			t.Errorf("plan route %q carries no protection-plans deny rule (rule %q)", key, requirement.Rule)
		}
	}
}

func TestRequestAllows(t *testing.T) {
	plans := roledata.ScopeProtectionPlans
	owner := map[string]roledata.PermissionLevel{plans: roledata.PermissionLevelOwner}
	rejectRule := xauthz.RuleKey(plans, roledata.ActionRejectProtectionPlan)

	cases := []struct {
		name          string
		identity      *xauthz.Identity
		wantApprove   bool
		wantRejection bool
	}{
		{
			name:          "internal peer",
			identity:      &xauthz.Identity{Internal: true},
			wantApprove:   true,
			wantRejection: true,
		},
		{
			name:          "owner",
			identity:      &xauthz.Identity{Grants: xauthz.Grants{Levels: owner}},
			wantApprove:   true,
			wantRejection: true,
		},
		{
			name: "owner denied reject",
			identity: &xauthz.Identity{Grants: xauthz.Grants{
				Levels: owner,
				Denied: map[string][]string{plans: {rejectRule}},
			}},
			wantApprove:   true,
			wantRejection: false,
		},
		{
			name: "contributor",
			identity: &xauthz.Identity{Grants: xauthz.Grants{
				Levels: map[string]roledata.PermissionLevel{plans: roledata.PermissionLevelContributor},
			}},
			wantApprove:   false,
			wantRejection: false,
		},
		{
			name:          "no identity",
			wantApprove:   false,
			wantRejection: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.identity != nil {
				ctx = xauthz.WithIdentity(ctx, *c.identity)
			}
			if got := authz.RequestAllows(ctx, authz.ApprovePlanRequirement()); got != c.wantApprove {
				t.Errorf("approve allowed = %v, want %v", got, c.wantApprove)
			}
			if got := authz.RequestAllows(ctx, authz.RejectPlanRequirement()); got != c.wantRejection {
				t.Errorf("reject allowed = %v, want %v", got, c.wantRejection)
			}
		})
	}
}
