package authz

import (
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	xauthz "github.com/telark/x-ware/authz"
)

// Create and the ledger routes are reached by discovery only; both lists and download
// follow the plan read scope, each with its own deny rule.
func TestReportRoutesRequirements(t *testing.T) {
	expected := map[string]xauthz.Requirement{
		"POST /api/v1/reports/plans/create":          xauthz.Internal,
		"POST /api/v1/reports/plans/{id}/ledger/put": xauthz.Internal,
		"GET /api/v1/reports/plans/{id}/ledger/get":  xauthz.Internal,
		"GET /api/v1/reports/get": xauthz.Denyable(
			xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
		),
		"GET /api/v1/reports/plans/{id}/get": xauthz.Denyable(
			xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
		),
		"GET /api/v1/reports/plans/{id}/download": xauthz.Denyable(
			xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionDownloadProtectionPlanReport,
		),
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
	}
}
