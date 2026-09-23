package planupdate

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	userID      = "u1"
	now         = "now"
	envA        = "e1"
	tagA        = "a"
	tagB        = "b"
	nameChanged = "changed"
)

func strptr(s string) *string { return &s }

func basePlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		Name:     "plan",
		Severity: plans.SeverityLow,
		Mode:     plans.ModeAudit,
		TimeMode: plans.TimeModePermanent,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"ns"}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}},
	}
}

func baseRequest(plan *plans.ProtectionPlan) *planseps.PrepareProtectionPlanRequest {
	return &planseps.PrepareProtectionPlanRequest{
		Name:     plan.Name,
		Severity: plan.Severity,
		Mode:     plan.Mode,
		TimeMode: plan.TimeMode,
		Scope:    planseps.ScopeRequest{Type: plan.Scope.Type, Namespaces: plan.Scope.Namespaces},
		Policies: []planseps.PolicyRequest{{TemplateID: "block-create"}},
	}
}

// An explicit "" / [] is a real clear: both keys are emitted as non-nil empties.
func TestBuildPatchTaxonomyClear(t *testing.T) {
	plan := basePlan()
	plan.EnvironmentID = envA
	plan.TagIDs = []string{tagA, tagB}
	req := baseRequest(plan)
	req.EnvironmentID = strptr("")
	req.TagIDs = []string{}

	patch, changed := update.BuildPatch(plan, req, plan.Policies, nil, userID, now)
	testutil.Equal(t, nameChanged, changed, true)
	if patch.TagIDs == nil {
		t.Fatal("tagIDs should be a non-nil empty slice on clear")
	}
	testutil.Equal(t, "tagIDs len", len(*patch.TagIDs), constants.DefaultInitValue)
	if patch.EnvironmentID == nil {
		t.Fatal("environmentID should be a non-nil empty string on clear")
	}
	testutil.Equal(t, "environmentID", *patch.EnvironmentID, "")
}

// A request without the keys keeps the stored values and reports no change.
func TestBuildPatchTaxonomyAbsentKeeps(t *testing.T) {
	plan := basePlan()
	plan.TagIDs = []string{tagA}
	req := baseRequest(plan)

	patch, changed := update.BuildPatch(plan, req, plan.Policies, plan.RenderedPolicies, userID, now)
	testutil.Equal(t, nameChanged, changed, false)
	if patch.TagIDs != nil {
		t.Fatalf("tagIDs should stay nil when absent, got %v", *patch.TagIDs)
	}
	if patch.EnvironmentID != nil {
		t.Fatalf("environmentID should stay nil when absent, got %q", *patch.EnvironmentID)
	}
}

func TestBuildPatchTaxonomySet(t *testing.T) {
	plan := basePlan()
	req := baseRequest(plan)
	req.EnvironmentID = strptr(envA)

	patch, changed := update.BuildPatch(plan, req, plan.Policies, nil, userID, now)
	testutil.Equal(t, nameChanged, changed, true)
	if patch.EnvironmentID == nil {
		t.Fatal("environmentID should be set")
	}
	testutil.Equal(t, "environmentID", *patch.EnvironmentID, envA)
}
