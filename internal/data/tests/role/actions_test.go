package role

import (
	"testing"

	roledata "github.com/telark/data/resources/role"
)

// Saved custom roles store these strings inside <scope>.<action>.deny, so renaming a
// value silently drops saved denials. Duplicate constant keys would not compile.
func TestProtectionPlanActionVocabulary(t *testing.T) {
	expected := map[string]string{
		roledata.ActionViewProtectionPlanViolations: "viewprotectionplanviolations",
		roledata.ActionCreateProtectionPlan:         "createprotectionplan",
		roledata.ActionEditProtectionPlan:           "editprotectionplan",
		roledata.ActionCancelProtectionPlan:         "cancelprotectionplan",
		roledata.ActionDuplicateProtectionPlan:      "duplicateprotectionplan",
		roledata.ActionReactivateProtectionPlan:     "reactivateprotectionplan",
		roledata.ActionDeleteProtectionPlan:         "deleteprotectionplan",
		roledata.ActionApproveProtectionPlan:        "approveprotectionplan",
		roledata.ActionViewProtectionPlans:          "viewprotectionplans",
		roledata.ActionRejectProtectionPlan:         "rejectprotectionplan",
		roledata.ActionGenerateProtectionPlanReport: "generateprotectionplanreport",
		roledata.ActionViewProtectionPlanReports:    "viewprotectionplanreports",
		roledata.ActionDownloadProtectionPlanReport: "downloadprotectionplanreport",
		roledata.ActionAddProtectionPlanCategory:    "addprotectionplancategory",
		roledata.ActionEditProtectionPlanCategory:   "editprotectionplancategory",
		roledata.ActionDeleteProtectionPlanCategory: "deleteprotectionplancategory",
	}
	for got, want := range expected {
		if got != want {
			t.Errorf("action %q, want %q", got, want)
		}
	}
}

func TestInsightsActionVocabulary(t *testing.T) {
	expected := map[string]string{
		roledata.ActionAnalyzeInsights: "analyzeinsights",
		roledata.ActionTriageInsights:  "triageinsights",
	}
	for got, want := range expected {
		if got != want {
			t.Errorf("action %q, want %q", got, want)
		}
	}
}
