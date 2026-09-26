package authz

import (
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

// The dashboard writes deny rules as "<scope>.<action>.deny" (formatRuleKey in
// the roles feature) and offers the action keys listed in its scopeRules. A
// rule saved there is only enforced if this service names the action
// identically, so the exact strings are pinned rather than left to agree by
// coincidence.
func TestRuleKeysMatchDashboardVocabulary(t *testing.T) {
	expected := map[string]string{
		"GET /api/v1/resources/applications/{name}/rollbacks/get": "applications.viewapplicationsrollbacks.deny",
		"GET /api/v1/snapshots/{id}/get":                          "applications.viewapplicationssnapshots.deny",
		"GET /api/v1/snapshots/{id}/manifest":                     "applications.viewapplicationsnapshotmanifest.deny",
		"PATCH /api/v1/resources/applications/{name}/patch":       "applications.editapplication.deny",
		"POST /api/v1/resources/users/create":                     "users.createuser.deny",
		"DELETE /api/v1/resources/users/{id}/delete":              "users.deleteuser.deny",
		"POST /api/v1/resources/groups/create":                    "groups.creategroup.deny",
		"PATCH /api/v1/resources/groups/{id}/patch":               "groups.editgroup.deny",
		"DELETE /api/v1/resources/groups/{id}/delete":             "groups.deletegroup.deny",
		"POST /api/v1/resources/roles/create":                     "roles.createrole.deny",
		"PATCH /api/v1/resources/roles/{id}/patch":                "roles.editrole.deny",
		"DELETE /api/v1/resources/roles/{id}/delete":              "roles.deleterole.deny",
		"GET /api/v1/plans/protection/get":                        "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/plans/protection/{id}/get":                   "protection-plans.viewprotectionplans.deny",
		"POST /api/v1/plans/protection/create":                    "protection-plans.createprotectionplan.deny",
		"PATCH /api/v1/plans/protection/{id}/patch":               "protection-plans.editprotectionplan.deny",
		"GET /api/v1/reports/plans/{id}/get":                      "protection-plans.viewprotectionplanreports.deny",
		"GET /api/v1/reports/plans/{id}/download":                 "protection-plans.downloadprotectionplanreport.deny",
	}

	requirements := authz.Requirements()
	for key, want := range expected {
		requirement, found := requirements[key]
		if !found {
			t.Errorf("route %q is not registered", key)
			continue
		}
		if requirement.Rule != want {
			t.Errorf("route %q rule = %q, want %q", key, requirement.Rule, want)
		}
	}
}

// A rule naming a scope other than its own could never match, since a role's
// rules are read from the scope's own list.
func TestRulesBelongToTheirOwnScope(t *testing.T) {
	for key, requirement := range authz.Requirements() {
		if requirement.Rule == "" {
			continue
		}
		want := xauthz.RuleKey(requirement.Scope, "")
		prefix := want[:len(requirement.Scope)+constants.DefaultIncrementValue]
		if len(requirement.Rule) < len(prefix) || requirement.Rule[:len(prefix)] != prefix {
			t.Errorf("route %q declares rule %q outside its scope %q", key, requirement.Rule, requirement.Scope)
		}
	}
}

func TestRuleKeyFormat(t *testing.T) {
	got := xauthz.RuleKey(roledata.ScopeApplications, roledata.ActionDeleteApplication)
	if want := "applications.deleteapplication.deny"; got != want {
		t.Errorf("RuleKey() = %q, want %q", got, want)
	}
}
