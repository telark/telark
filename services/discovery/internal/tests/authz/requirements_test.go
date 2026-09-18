package authz

import (
	"testing"

	"github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/routes"
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

	if len(missing) > 0 {
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
// scope check.
func TestNoRouteSkipsScopeCheck(t *testing.T) {
	for key, requirement := range authz.Requirements() {
		if requirement.Access == xauthz.AccessAuthenticated {
			t.Errorf("route %q requires only a session and no scope", key)
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
		if requirement.MinLevel.Rank() == 0 {
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
		if requirement.MinLevel.Rank() < 2 {
			t.Errorf("destructive route %q only needs %q", key, requirement.MinLevel)
		}
	}
}
