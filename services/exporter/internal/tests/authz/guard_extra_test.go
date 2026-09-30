package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	categorydata "github.com/telark/telark/internal/data/classification/category"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const testSessionToken = "tok"

func noIdentityRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/", nil)
}

func TestGuardCategoryScope(t *testing.T) {
	operation := constants.CategoryOpCreate
	tests := []struct {
		name  string
		req   *http.Request
		scope string
		want  bool
	}{
		{"missing identity", noIdentityRequest(), roledata.ScopeRoles, false},
		{"internal bypasses", requestAs(xauthz.Identity{Internal: true}), roledata.ScopeRoles, true},
		{"unknown scope denied", requestAs(xauthz.Identity{UserID: "u1"}), "galaxy", false},
		{"known scope without grant denied", requestAs(xauthz.Identity{UserID: "u1"}), roledata.ScopeRoles, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardCategoryScope(w, tt.req, tt.scope, operation); got != tt.want {
				t.Errorf("GuardCategoryScope = %v, want %v", got, tt.want)
			}
		})
	}

	w := httptest.NewRecorder()
	admin := levels(roledata.ScopeRoles, roledata.PermissionLevelAdmin)
	if authz.GuardCategoryScope(w, requestAs(admin), roledata.ScopeRoles, "rename") {
		t.Error("an unknown operation passed the category guard")
	}
	if body := w.Body.String(); !strings.Contains(body, constants.ErrAuthzUnknownCategoryScope) {
		t.Errorf("unknown operation body = %q, want %q", body, constants.ErrAuthzUnknownCategoryScope)
	}
}

func TestGuardSelfSessionToken(t *testing.T) {
	w := httptest.NewRecorder()
	if authz.GuardSelfSessionToken(w, noIdentityRequest(), testSessionToken) {
		t.Error("session guard passed without identity")
	}

	if !authz.GuardSelfSessionToken(httptest.NewRecorder(), requestAs(xauthz.Identity{Internal: true}), testSessionToken) {
		t.Error("internal caller should pass session guard")
	}

	// A non-internal caller cannot resolve the token (no session backend), so it is denied.
	if authz.GuardSelfSessionToken(httptest.NewRecorder(), requestAs(xauthz.Identity{UserID: "u1"}), testSessionToken) {
		t.Error("unresolvable token should be denied")
	}
}

func levels(scope string, level roledata.PermissionLevel) xauthz.Identity {
	return xauthz.Identity{
		UserID: "u1",
		Grants: xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: level}},
	}
}

func denied(id xauthz.Identity, scope, rule string) xauthz.Identity {
	id.Grants.Denied = map[string][]string{scope: {rule}}
	return id
}

// Plan taxonomies are governed by the protection-plans scope and their own
// category actions; nothing passes through from a grant on the taxonomy name itself.
func TestGuardCategoryScopePlanTaxonomies(t *testing.T) {
	contributor, owner := roledata.PermissionLevelContributor, roledata.PermissionLevelOwner
	plans, groups, roles := roledata.ScopeProtectionPlans, roledata.ScopeGroups, roledata.ScopeRoles
	environments, tags := categorydata.ScopePlanEnvironments, categorydata.ScopePlanTags
	create, edit, remove := constants.CategoryOpCreate, constants.CategoryOpEdit, constants.CategoryOpDelete
	ownerDenied := func(action string) xauthz.Identity {
		return denied(levels(plans, owner), plans, xauthz.RuleKey(plans, action))
	}
	groupOwnerNoEdit := denied(levels(groups, owner), groups, xauthz.RuleKey(groups, roledata.ActionEditGroupCategory))
	groupOwnerNoDelete := denied(levels(groups, owner), groups, xauthz.RuleKey(groups, roledata.ActionDeleteGroupCategory))
	roleOwnerNoDelete := denied(levels(roles, owner), roles, xauthz.RuleKey(roles, roledata.ActionDeleteRoleCategory))
	tests := []struct {
		name      string
		id        xauthz.Identity
		scope     string
		operation string
		want      bool
	}{
		{"plans contributor adds environment", levels(plans, contributor), environments, create, true},
		{"plans contributor edits tag", levels(plans, contributor), tags, edit, false},
		{"plans owner edits environment", levels(plans, owner), environments, edit, true},
		{"plans owner deletes tag", levels(plans, owner), tags, remove, true},
		{"add deny bites tag add", ownerDenied(roledata.ActionAddProtectionPlanCategory), tags, create, false},
		{"edit deny bites tag edit", ownerDenied(roledata.ActionEditProtectionPlanCategory), tags, edit, false},
		{"edit deny spares tag delete", ownerDenied(roledata.ActionEditProtectionPlanCategory), tags, remove, true},
		{"delete deny bites environment delete", ownerDenied(roledata.ActionDeleteProtectionPlanCategory), environments, remove, false},
		{"plan edit deny no longer governs taxonomies", ownerDenied(roledata.ActionEditProtectionPlan), environments, edit, true},
		{"plan create deny no longer governs taxonomies", ownerDenied(roledata.ActionCreateProtectionPlan), tags, create, true},
		{"groups grant does not pass through", levels(groups, owner), environments, create, false},
		{"groups category still governed by groups", levels(groups, contributor), groups, create, true},
		{"group edit deny spares group category delete", groupOwnerNoEdit, groups, remove, true},
		{"group delete deny bites group category delete", groupOwnerNoDelete, groups, remove, false},
		{"role delete deny bites role category delete", roleOwnerNoDelete, roles, remove, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardCategoryScope(w, requestAs(tt.id), tt.scope, tt.operation); got != tt.want {
				t.Fatalf("GuardCategoryScope = %v, want %v", got, tt.want)
			}
			if tt.want {
				return
			}
			body := w.Body.String()
			if !strings.Contains(body, constants.ErrAuthzCategoryScopeDenied) || strings.Contains(body, constants.ErrAuthzUnknownCategoryScope) {
				t.Errorf("denied body = %q, want %q", body, constants.ErrAuthzCategoryScopeDenied)
			}
		})
	}
}
