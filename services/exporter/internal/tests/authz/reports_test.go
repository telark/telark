package authz

import (
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

// Create and the ledger routes are reached by discovery only; list and
// download follow the plan read scope like the discovery violations route.
func TestReportRoutesRequirements(t *testing.T) {
	expected := map[string]xauthz.Requirement{
		"POST /api/v1/reports/plans/create":          xauthz.Internal,
		"POST /api/v1/reports/plans/{id}/ledger/put": xauthz.Internal,
		"GET /api/v1/reports/plans/{id}/ledger/get":  xauthz.Internal,
		"GET /api/v1/reports/plans/{id}/get":         xauthz.Read(roledata.ScopeProtectionPlans),
		"GET /api/v1/reports/plans/{id}/download":    xauthz.Read(roledata.ScopeProtectionPlans),
	}

	requirements := authz.Requirements()
	for key, want := range expected {
		got, found := requirements[key]
		if !found {
			t.Errorf("report route %q has no requirement", key)
			continue
		}
		if got != want {
			t.Errorf("report route %q = %+v, want %+v", key, got, want)
		}
		if got.Rule != constants.EmptyString {
			t.Errorf("report route %q carries rule %q, want none", key, got.Rule)
		}
	}
}
