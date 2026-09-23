package authz

import (
	"testing"

	"github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/routes"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/rest/base"
	autheps "github.com/telark/rest/endpoints/auth"
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

// The login surface is public by necessity. Anything else that is public is a
// mistake, so the set is pinned here rather than left to review.
func TestPublicRoutesArePinned(t *testing.T) {
	allowed := map[string]bool{
		"GET /api/v1/status/health":               true,
		"GET /api/v1/status/ready":                true,
		"GET /api/v1/status/live":                 true,
		"POST /api/v1/auth/login/start":           true,
		"POST /api/v1/auth/login/finish":          true,
		"POST /api/v1/auth/register/start":        true,
		"POST /api/v1/auth/passkeys/proxy/create": true,
		"GET /api/v1/auth/config":                 true,
		"POST /api/v1/auth/logout":                true,
		"POST /api/v1/auth/oidc/google/callback":  true,
		"POST /api/v1/auth/oidc/google/nonce":     true,
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
