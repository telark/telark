package authz

import (
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
)

// Create and the ledger routes are reached by discovery only; both lists and download
// follow the plan read scope, each with its own deny rule.
func TestReportRoutesRequirements(t *testing.T) {
	expected := map[string]xauthz.Requirement{
		"POST /api/v1/internal/reports":                    xauthz.Internal,
		"PUT /api/v1/internal/protectionplans/{id}/ledger": xauthz.Internal,
		"GET /api/v1/internal/protectionplans/{id}/ledger": xauthz.Internal,
		"GET /api/v1/reports": xauthz.Denyable(
			xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
		),
		"GET /api/v1/protectionplans/{id}/reports": xauthz.Denyable(
			xauthz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
		),
		"GET /api/v1/protectionplans/{id}/reports/download": xauthz.Denyable(
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
