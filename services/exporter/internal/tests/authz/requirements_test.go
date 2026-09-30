package authz

import (
	"strings"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/performance"
	xauthz "github.com/telark/x-ware/authz"
)

// Every route must carry a rule. Without this the next endpoint someone adds
// is silently unreachable (default-deny) or, worse, someone "fixes" that by
// loosening the default.
func TestRequirementsCoverEveryRoute(t *testing.T) {
	requirements := authz.Requirements()

	var missing []string
	for _, route := range routes.InitRoutes(&performance.Optimizer{}) {
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

// A stale key protects nothing and hides a typo in RouteKey.
func TestNoRequirementWithoutRoute(t *testing.T) {
	registered := map[string]bool{}
	for _, route := range routes.InitRoutes(&performance.Optimizer{}) {
		registered[route.Method+" "+route.Pattern] = true
	}

	for key := range authz.Requirements() {
		if !registered[key] {
			t.Errorf("requirement %q matches no registered route", key)
		}
	}
}

// Public means unauthenticated. Only probes qualify.
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

// A route taking no scope is only safe if something narrows it afterwards: a
// self guard for a user's own records, or GuardCategoryScope for a category,
// whose governing scope is a property of the record and not of the route.
// Pinning the set forces every addition to be a deliberate decision.
func TestScopelessRoutesAreGuarded(t *testing.T) {
	expected := map[string]bool{
		// Narrowed to the caller by GuardSelfUser / GuardSelfSessionToken.
		"GET /api/v1/auth/sessions":         true,
		"GET /api/v1/auth/sessions/self":    true,
		"DELETE /api/v1/auth/sessions/self": true,
		// Narrowed to the caller's own sessions by GuardOwnSessionName.
		"DELETE /api/v1/auth/sessions/{name}":  true,
		"GET /api/v1/notifications":            true,
		"POST /api/v1/notifications/{id}/read": true,
		"POST /api/v1/notifications/read":      true,
		"DELETE /api/v1/notifications":         true,
		"DELETE /api/v1/notifications/{id}":    true,
		// Narrowed to the category's own scope by GuardCategoryScope.
		"POST /api/v1/categories":        true,
		"PATCH /api/v1/categories/{id}":  true,
		"DELETE /api/v1/categories/{id}": true,
		// Reference data needed to render any list (?scope= narrows the list).
		"GET /api/v1/categories":      true,
		"GET /api/v1/categories/{id}": true,
		// Narrowed field by field by GuardConfigPatch.
		"PATCH /api/v1/config": true,
		// Narrowed to the profile owner, and per privileged field, by GuardUserPatch.
		"PATCH /api/v1/users/{id}": true,
	}

	for key, requirement := range authz.Requirements() {
		if requirement.Access == xauthz.AccessAuthenticated && !expected[key] {
			t.Errorf("route %q takes no scope and is not a known guarded route", key)
		}
	}
}

// Protection plans are their own feature, not part of applications.
func TestProtectionPlansUseTheirOwnScope(t *testing.T) {
	planRoutes := []string{
		"GET /api/v1/protectionplans",
		"GET /api/v1/protectionplans/{id}",
		"POST /api/v1/protectionplans",
	}

	requirements := authz.Requirements()
	for _, key := range planRoutes {
		requirement, found := requirements[key]
		if !found {
			t.Errorf("plan route %q has no requirement", key)
			continue
		}
		if requirement.Scope != roledata.ScopeProtectionPlans {
			t.Errorf("plan route %q uses scope %q, want %q", key, requirement.Scope, roledata.ScopeProtectionPlans)
		}
	}
}

// Users delete through discovery's clear route, which removes the deployed
// policies first; a session reaching the CR delete would leave them enforcing.
// Edits likewise go through discovery, which owns the plan lifecycle.
func TestPlanDeleteAndPatchAreInternal(t *testing.T) {
	for _, key := range []string{
		"DELETE /api/v1/protectionplans/{id}",
		"PATCH /api/v1/protectionplans/{id}",
	} {
		if got := authz.Requirements()[key]; got != xauthz.Internal {
			t.Errorf("plan route %q = %+v, want Internal", key, got)
		}
	}
}

// Applications and their snapshots are authored by discovery and the notifier;
// a session reaching these would forge state or leave discovery's Redis stale.
func TestApplicationAuthoringRoutesAreInternal(t *testing.T) {
	keys := []string{
		"POST /api/v1/applications",
		"DELETE /api/v1/applications/{name}",
		"POST /api/v1/internal/snapshots",
		"DELETE /api/v1/snapshots/{id}",
	}
	requirements := authz.Requirements()
	for _, key := range keys {
		if got := requirements[key]; got != xauthz.Internal {
			t.Errorf(wantInternalFmt, key, got)
		}
	}
}

const wantInternalFmt = "route %q = %+v, want Internal"

// The internal/ prefix is the contract peers and the UI's nginx rely on: every
// route under it is service-only, and a gateway can refuse the whole prefix.
func TestInternalPrefixRoutesAreInternal(t *testing.T) {
	var found int
	for key, requirement := range authz.Requirements() {
		if !strings.Contains(key, " /api/v1/internal/") {
			continue
		}
		found++
		if requirement != xauthz.Internal {
			t.Errorf(wantInternalFmt, key, requirement)
		}
	}
	if found == constants.DefaultInitValue {
		t.Fatal("no internal/ route found; the scan is broken")
	}
}

// Service-only lookups moved under internal/ so no query variant of a user
// route can share the list route's requirement.
func TestServiceOnlyRoutesAreInternal(t *testing.T) {
	for _, key := range []string{
		"GET /api/v1/internal/users/by-username/{username}",
		"GET /api/v1/internal/users/by-email/{email}",
		"GET /api/v1/internal/users/by-identity",
		"POST /api/v1/internal/auth/users/{userId}/sessions",
		"PATCH /api/v1/auth/sessions/self",
		"POST /api/v1/internal/auth/passkeys",
		"GET /api/v1/internal/auth/passkeys",
		"GET /api/v1/internal/auth/passkeys/{credentialId}",
		"PATCH /api/v1/internal/auth/passkeys/{credentialId}",
		"DELETE /api/v1/internal/auth/passkeys/{credentialId}",
		"POST /api/v1/internal/notifications",
	} {
		if got := authz.Requirements()[key]; got != xauthz.Internal {
			t.Errorf(wantInternalFmt, key, got)
		}
	}
}

// Only auth's cleanup cascade sets finalizers: a session could pin a record
// forever, or drop one and skip the cascade.
func TestFinalizerRoutesAreInternal(t *testing.T) {
	for _, key := range []string{
		"PUT /api/v1/cleanup/{type}/{id}/finalizer",
		"DELETE /api/v1/cleanup/{type}/{id}/finalizer",
	} {
		if got := authz.Requirements()[key]; got != xauthz.Internal {
			t.Errorf(wantInternalFmt, key, got)
		}
	}
}

// A scoped rule with no scope or no level can only ever deny, which would be a
// silent outage rather than a policy.
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
