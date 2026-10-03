package authz

import (
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/routes"
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

// The login surface is public by necessity. Anything else that is public is a
// mistake, so the set is pinned here rather than left to review.
func TestPublicRoutesArePinned(t *testing.T) {
	allowed := map[string]bool{
		"GET /api/v1/status/health":              true,
		"GET /api/v1/status/ready":               true,
		"GET /api/v1/status/live":                true,
		"POST /api/v1/auth/login/start":          true,
		"POST /api/v1/auth/login/finish":         true,
		"POST /api/v1/auth/register/start":       true,
		"POST /api/v1/auth/passkeys":             true,
		"GET /api/v1/auth/config":                true,
		"POST /api/v1/auth/logout":               true,
		"POST /api/v1/auth/oidc/google/callback": true,
		"POST /api/v1/auth/oidc/google/nonce":    true,
	}

	for key, requirement := range authz.Requirements() {
		if requirement.Access == xauthz.AccessPublic && !allowed[key] {
			t.Errorf("route %q is public but is not part of the login surface", key)
		}
	}
}

// The exporter writes this config but stands aside for the service token on the way
// in, so this requirement is the only thing standing between a caller and control of
// who can authenticate. Pinned rather than left to review.
func TestOIDCConfigRequiresAdmin(t *testing.T) {
	key := router.Key(base.Patch, autheps.OIDCConfig)
	requirement, found := authz.Requirements()[key]
	if !found {
		t.Fatalf("no requirement for %q", key)
	}

	if requirement.Access != xauthz.AccessScoped {
		t.Errorf("access = %v, want scoped", requirement.Access)
	}
	if requirement.Scope != roledata.ScopeSettings {
		t.Errorf("scope = %q, want %q", requirement.Scope, roledata.ScopeSettings)
	}
	if requirement.MinLevel != roledata.PermissionLevelAdmin {
		t.Errorf("level = %q, want %q", requirement.MinLevel, roledata.PermissionLevelAdmin)
	}

	want := xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditOIDCConfig)
	if requirement.Rule != want {
		t.Errorf("rule = %q, want %q", requirement.Rule, want)
	}
}

// Both handlers then apply rules of their own (the enroll-link target rules, the bootstrap
// account), so these route levels are only the floor; pinned rather than left to review.
func TestInviteAndSelfRegistrationRequirements(t *testing.T) {
	cases := []struct {
		method   base.Method
		endpoint base.Endpoint
		scope    string
		level    roledata.PermissionLevel
	}{
		{base.Post, autheps.UserEnrollLink, roledata.ScopeUsers, roledata.PermissionLevelOwner},
		{base.Delete, autheps.UserEnrollLink, roledata.ScopeUsers, roledata.PermissionLevelOwner},
		{base.Patch, autheps.SelfRegistration, roledata.ScopeSettings, roledata.PermissionLevelAdmin},
	}
	requirements := authz.Requirements()
	for _, c := range cases {
		key := router.Key(c.method, c.endpoint)
		requirement, found := requirements[key]
		if !found {
			t.Errorf("%q has no requirement", key)
			continue
		}
		if requirement.Access != xauthz.AccessScoped || requirement.Scope != c.scope || requirement.MinLevel != c.level {
			t.Errorf("%s: requirement = %+v, want %s on %s", key, requirement, c.level, c.scope)
		}
	}
}

// The cleanup handler deletes through the exporter with the service token, which
// the exporter's guard waves through, so the delete deny rules only bite here.
func TestCleanupDeletesHonorDenyRules(t *testing.T) {
	cases := []struct {
		endpoint      base.Endpoint
		scope, action string
	}{
		{autheps.DeleteUserCleanup, roledata.ScopeUsers, roledata.ActionDeleteUser},
		{autheps.DeleteGroupCleanup, roledata.ScopeGroups, roledata.ActionDeleteGroup},
		{autheps.DeleteAccessRoleCleanup, roledata.ScopeRoles, roledata.ActionDeleteRole},
	}
	requirements := authz.Requirements()
	for _, c := range cases {
		key := router.Key(base.Delete, c.endpoint)
		requirement, found := requirements[key]
		if !found {
			t.Fatalf("no requirement for %q", key)
		}
		if requirement.Scope != c.scope || requirement.MinLevel != roledata.PermissionLevelOwner {
			t.Errorf("%s: scope/level = %q/%q, want %q/Owner", key, requirement.Scope, requirement.MinLevel, c.scope)
		}
		if want := xauthz.RuleKey(c.scope, c.action); requirement.Rule != want {
			t.Errorf("%s: rule = %q, want %q", key, requirement.Rule, want)
		}
	}
}

func TestScopedRequirementsAreComplete(t *testing.T) {
	for key, requirement := range authz.Requirements() {
		if requirement.Access != xauthz.AccessScoped {
			continue
		}
		if requirement.Scope == "" {
			t.Errorf("route %q is scoped but declares no scope", key)
		}
		if requirement.MinLevel.Rank() == constants.DefaultInitValue {
			t.Errorf("route %q declares an unusable level %q", key, requirement.MinLevel)
		}
	}
}

const routeKeySeparator = " "

// The route table is the public contract: the new paths carry the credential id and
// the resource id in the path, and the retired ones must be gone from routes and map.
func TestRenamedRoutesArePinned(t *testing.T) {
	requirements := authz.Requirements()
	want := map[string]xauthz.Access{
		"GET /api/v1/auth/passkeys":                   xauthz.AccessAuthenticated,
		"POST /api/v1/auth/passkeys":                  xauthz.AccessPublic,
		"GET /api/v1/auth/passkeys/{credentialId}":    xauthz.AccessAuthenticated,
		"PATCH /api/v1/auth/passkeys/{credentialId}":  xauthz.AccessAuthenticated,
		"DELETE /api/v1/auth/passkeys/{credentialId}": xauthz.AccessAuthenticated,
		"POST /api/v1/auth/passkeys/enroll-link":      xauthz.AccessAuthenticated,
		"DELETE /api/v1/auth/users/{id}":              xauthz.AccessScoped,
		"DELETE /api/v1/auth/groups/{id}":             xauthz.AccessScoped,
		"DELETE /api/v1/auth/accessroles/{id}":        xauthz.AccessScoped,
		"PATCH /api/v1/auth/oidc/config":              xauthz.AccessScoped,
	}
	for key, access := range want {
		requirement, found := requirements[key]
		if !found {
			t.Errorf("missing requirement %q", key)
			continue
		}
		if requirement.Access != access {
			t.Errorf("%s: access = %v, want %v", key, requirement.Access, access)
		}
	}

	registered := map[string]bool{}
	for _, route := range routes.Routes {
		registered[route.Method+routeKeySeparator+route.Pattern] = true
	}
	retired := []string{
		"GET /api/v1/auth/passkeys/proxy/get",
		"POST /api/v1/auth/passkeys/proxy/create",
		"GET /api/v1/auth/passkeys/proxy/single/get",
		"PATCH /api/v1/auth/passkeys/proxy/patch",
		"DELETE /api/v1/auth/passkeys/proxy/delete",
		"DELETE /api/v1/auth/users/{id}/cleanup",
		"DELETE /api/v1/auth/groups/{id}/cleanup",
		"DELETE /api/v1/auth/roles/{id}/cleanup",
	}
	for _, key := range retired {
		if _, found := requirements[key]; found || registered[key] {
			t.Errorf("retired route %q is still served", key)
		}
	}
}
