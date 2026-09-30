package authz

import (
	"context"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	clustereps "github.com/telark/telark/internal/rest/endpoints/cluster"
	insightseps "github.com/telark/telark/internal/rest/endpoints/insights"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/routes"
)

const routeKeySeparator = " "

func TestRequirementsCoverEveryRoute(t *testing.T) {
	requirements := authz.Requirements()

	var missing []string
	for _, route := range routes.Routes {
		key := route.Method + routeKeySeparator + route.Pattern
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
		registered[route.Method+routeKeySeparator+route.Pattern] = true
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
	handlerChecked := router.Key(base.Get, clustereps.GetAllNamespaces)
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
		"POST /api/v1/applications/{name}/rollbacks",
		"POST /api/v1/applications/{name}/rollbacks/{rollbackId}/abort",
		"POST /api/v1/applications/{name}/sync",
		"POST /api/v1/applications/{name}/reset",
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
		"GET /api/v1/policytemplates":                  "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/protectionplans/{id}/status":      "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/protectionplans/{id}/violations":  "protection-plans.viewprotectionplanviolations.deny",
		"POST /api/v1/protectionplans/prepare":         "protection-plans.createprotectionplan.deny",
		"POST /api/v1/protectionplans/{id}/revise":     "protection-plans.editprotectionplan.deny",
		"POST /api/v1/protectionplans/{id}/duplicate":  "protection-plans.duplicateprotectionplan.deny",
		"POST /api/v1/protectionplans/{id}/reactivate": "protection-plans.reactivateprotectionplan.deny",
		"POST /api/v1/protectionplans/{id}/cancel":     "protection-plans.cancelprotectionplan.deny",
		"POST /api/v1/protectionplans/{id}/reports":    "protection-plans.generateprotectionplanreport.deny",
		"DELETE /api/v1/protectionplans/{id}/clear":    "protection-plans.deleteprotectionplan.deny",
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

// The route table is the public contract the dashboard and the analyzer call, so
// it is pinned literally in both directions: no route disappears or renames
// silently, and no route appears without a line here.
var wantRouteTable = map[string]xauthz.Requirement{
	"GET /api/v1/status/live":  xauthz.Public,
	"GET /api/v1/status/ready": xauthz.Public,

	"GET /api/v1/cluster/namespaces":                       xauthz.Authenticated,
	"GET /api/v1/cluster/namespaces/{namespace}/workloads": xauthz.Read(roledata.ScopeApplications),
	"GET /api/v1/cluster/namespaces/{namespace}/resources": xauthz.Read(roledata.ScopeApplications),

	"GET /api/v1/discovery/status": xauthz.Read(roledata.ScopeApplications),
	"POST /api/v1/applications/{name}/rollbacks": xauthz.Denyable(
		xauthz.Write(roledata.ScopeApplications), roledata.ActionRollbackApplication),
	"POST /api/v1/applications/{name}/rollbacks/{rollbackId}/abort": xauthz.Denyable(
		xauthz.Write(roledata.ScopeApplications), roledata.ActionRollbackApplication),
	"POST /api/v1/applications/{name}/sync": xauthz.Denyable(
		xauthz.Write(roledata.ScopeApplications), roledata.ActionForceApplicationSync),
	"POST /api/v1/applications/{name}/reset": xauthz.Denyable(
		xauthz.Own(roledata.ScopeApplications), roledata.ActionDeleteApplication),

	"GET /api/v1/insights/applications": xauthz.Read(roledata.ScopeInsights),
	"GET /api/v1/insights":              xauthz.Read(roledata.ScopeInsights),

	"GET /api/v1/policytemplates": xauthz.Denyable(
		xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans),
	"GET /api/v1/protectionplans/{id}/status": xauthz.Denyable(
		xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans),
	"GET /api/v1/protectionplans/{id}/violations": xauthz.Denyable(
		xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanViolations),
	"POST /api/v1/protectionplans/prepare": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan),
	"POST /api/v1/protectionplans/{id}/cancel": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionCancelProtectionPlan),
	"POST /api/v1/protectionplans/{id}/duplicate": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionDuplicateProtectionPlan),
	"POST /api/v1/protectionplans/{id}/reactivate": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionReactivateProtectionPlan),
	"POST /api/v1/protectionplans/{id}/revise": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlan),
	"POST /api/v1/protectionplans/{id}/decision": xauthz.Own(roledata.ScopeProtectionPlans),
	"POST /api/v1/protectionplans/{id}/reports": xauthz.Denyable(
		xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionGenerateProtectionPlanReport),
	"DELETE /api/v1/protectionplans/{id}/clear": xauthz.Denyable(
		xauthz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlan),
}

func TestRouteTableMatchesContract(t *testing.T) {
	requirements := authz.Requirements()
	registered := map[string]bool{}
	for _, route := range routes.Routes {
		registered[route.Method+routeKeySeparator+route.Pattern] = true
	}

	for key, want := range wantRouteTable {
		if !registered[key] {
			t.Errorf("contract route %q is not registered", key)
		}
		if got, found := requirements[key]; !found || got != want {
			t.Errorf("route %q requirement = %+v (found %v), want %+v", key, got, found, want)
		}
	}
	for key := range registered {
		if _, found := wantRouteTable[key]; !found {
			t.Errorf("registered route %q is missing from the contract table", key)
		}
	}
	for key := range requirements {
		if _, found := wantRouteTable[key]; !found {
			t.Errorf("requirement %q is missing from the contract table", key)
		}
	}
}

// The old verb-suffixed and resources/-prefixed paths must not survive as aliases.
func TestNoLegacyRoutePaths(t *testing.T) {
	legacy := []string{"/resources/", "/plans/protection", "/analyze/", "/get", "/create", "/update", "/decide", "/trigger", "/generate"}
	for _, route := range routes.Routes {
		for _, fragment := range legacy {
			if strings.Contains(route.Pattern, fragment) {
				t.Errorf("route %s %s keeps the legacy fragment %q", route.Method, route.Pattern, fragment)
			}
		}
	}
}
