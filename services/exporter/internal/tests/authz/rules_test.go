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
		"GET /api/v1/applications/{name}/rollbacks":         "applications.viewapplicationsrollbacks.deny",
		"GET /api/v1/snapshots/{id}":                        "applications.viewapplicationssnapshots.deny",
		"GET /api/v1/snapshots/{id}/manifest":               "applications.viewapplicationsnapshotmanifest.deny",
		"PATCH /api/v1/applications/{name}":                 "applications.editapplication.deny",
		"POST /api/v1/users":                                "users.createuser.deny",
		"DELETE /api/v1/users/{id}":                         "users.deleteuser.deny",
		"POST /api/v1/groups":                               "groups.creategroup.deny",
		"PATCH /api/v1/groups/{id}":                         "groups.editgroup.deny",
		"DELETE /api/v1/groups/{id}":                        "groups.deletegroup.deny",
		"POST /api/v1/accessroles":                          "roles.createrole.deny",
		"PATCH /api/v1/accessroles/{id}":                    "roles.editrole.deny",
		"DELETE /api/v1/accessroles/{id}":                   "roles.deleterole.deny",
		"GET /api/v1/protectionplans":                       "protection-plans.viewprotectionplans.deny",
		"GET /api/v1/protectionplans/{id}":                  "protection-plans.viewprotectionplans.deny",
		"POST /api/v1/protectionplans":                      "protection-plans.createprotectionplan.deny",
		"GET /api/v1/protectionplans/{id}/reports":          "protection-plans.viewprotectionplanreports.deny",
		"GET /api/v1/protectionplans/{id}/reports/download": "protection-plans.downloadprotectionplanreport.deny",
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
