package planupdate

import (
	"reflect"
	"slices"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	restmapper "github.com/telark/rest/mappers"
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

func TestBuildPatchNeverEmitsApprovalKeys(t *testing.T) {
	plan := basePlan()
	plan.ApprovalMode = plans.ApprovalModeRequired
	plan.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStatePending, RequestedBy: userID, RequestedAt: now}
	req := baseRequest(plan)
	req.Name = nameChanged
	req.ApprovalMode = strptr(plans.ApprovalModeAutomatic)

	patch, _ := update.BuildPatch(plan, req, plan.Policies, nil, userID, now)
	body, err := restmapper.MapToJSONPayload(patch)
	if err != nil {
		t.Fatalf("map patch: %v", err)
	}
	for _, key := range []string{protection.FieldApproval, protection.FieldApprovalMode} {
		if _, found := body[key]; found {
			t.Fatalf("patch must never carry %q, got %v", key, body[key])
		}
	}
}

// Exclusions are persisted inside scope, so an exclusions-only edit must still patch scope,
// carrying the normalized exclusions and the stored targets.
func TestBuildPatchScopeOnExclusionsOnlyChange(t *testing.T) {
	plan := basePlan()
	req := baseRequest(plan)
	req.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindSecret, kindConfigMap, kindSecret}}

	patch, changed := update.BuildPatch(plan, req, plan.Policies, plan.RenderedPolicies, userID, now)
	testutil.Equal(t, nameChanged, changed, true)
	if patch.Scope == nil {
		t.Fatal("scope should be patched on an exclusions-only change")
	}
	if want := storedKinds(); !reflect.DeepEqual(patch.Scope.Exclusions, want) {
		t.Fatalf("exclusions = %+v, want normalized %+v", patch.Scope.Exclusions, want)
	}
	if !slices.Equal(patch.Scope.Namespaces, plan.Scope.Namespaces) {
		t.Fatalf("namespaces = %v, want stored %v", patch.Scope.Namespaces, plan.Scope.Namespaces)
	}
}

// An explicit empty object clears: scope is patched and the effective exclusions are nil.
func TestBuildPatchScopeOnExclusionsClear(t *testing.T) {
	plan := basePlan()
	plan.Scope.Exclusions = storedKinds()
	req := baseRequest(plan)
	req.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{}

	patch, changed := update.BuildPatch(plan, req, plan.Policies, plan.RenderedPolicies, userID, now)
	testutil.Equal(t, nameChanged, changed, true)
	if patch.Scope == nil || patch.Scope.Exclusions != nil {
		t.Fatalf("scope = %+v, want a scope patch with nil exclusions", patch.Scope)
	}
}

// A request without exclusions leaves them untouched, so a targets change carries the stored ones.
func TestBuildPatchScopeCarriesStoredExclusionsWhenTargetsChange(t *testing.T) {
	plan := basePlan()
	plan.Scope.Exclusions = storedKinds()
	req := baseRequest(plan)
	req.Scope.Namespaces = []string{otherNS}

	patch, _ := update.BuildPatch(plan, req, plan.Policies, plan.RenderedPolicies, userID, now)
	if patch.Scope == nil {
		t.Fatal("scope should be patched when targets change")
	}
	if !reflect.DeepEqual(patch.Scope.Exclusions, plan.Scope.Exclusions) {
		t.Fatalf("exclusions = %+v, want stored %+v", patch.Scope.Exclusions, plan.Scope.Exclusions)
	}
}

func TestBuildPatchNoScopeWhenNothingChanged(t *testing.T) {
	plan := basePlan()
	plan.Scope.Exclusions = storedKinds()
	req := baseRequest(plan)
	req.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindSecret, kindConfigMap}}

	patch, changed := update.BuildPatch(plan, req, plan.Policies, plan.RenderedPolicies, userID, now)
	testutil.Equal(t, nameChanged, changed, false)
	if patch.Scope != nil {
		t.Fatalf("scope should not be patched, got %+v", patch.Scope)
	}
}
