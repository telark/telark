package planduplicate

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/duplicate"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	sourceEnvironmentRef   = "cat-00002-0001-0001"
	overrideEnvironmentRef = "cat-00002-0001-0002"
	sourceTagID            = "cat-00003-0001-0001"
	overrideTagA           = "cat-00003-0001-0002"
	overrideTagB           = "cat-00003-0001-0003"
	defaultCopyName        = "Copy of prod-guard"
	secondCopyName         = "Copy of prod-guard (2)"
	takenSuffixCeiling     = 200
	labelApprovalMode      = "approvalMode"
	requesterID            = "u1"
)

func strptr(s string) *string { return &s }

func sourcePlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		Name:            "prod-guard",
		Severity:        "high",
		Priority:        constants.TwoValue,
		Mode:            plans.ModeEnforce,
		TimeMode:        plans.TimeModeTimeRange,
		Scope:           plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}},
		Policies:        []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}},
		TimeRange:       &plans.ProtectionPlanTimeRange{StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z"},
		ParticipantRefs: []string{requesterID},
		EnvironmentRef:  sourceEnvironmentRef,
		TagRefs:         []string{sourceTagID},
	}
}

// With no overrides the copy is named "Copy of <source>", carries the source
// scope and policies, and reuses the source time range.
func TestBuildRequestDefaults(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{})
	testutil.Equal(t, "name", got.Name, defaultCopyName)
	testutil.Equal(t, "scope type", got.Scope.Type, plans.ScopeTypeNamespaces)
	testutil.Equal(t, "policies", len(got.Policies), constants.DefaultAddValue)
	testutil.Equal(t, "policy template", got.Policies[0].TemplateID, "block-create")
	if got.TimeRange == nil || got.TimeRange.StartAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("time range = %+v, want source window", got.TimeRange)
	}
}

// Overrides win: an explicit name and time range replace the source values.
func TestBuildRequestOverrides(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		Name:      strptr("My Copy"),
		TimeRange: &planseps.TimeRangeRequest{StartAt: "2026-02-01T00:00:00Z", EndAt: "2026-02-02T00:00:00Z"},
	})
	testutil.Equal(t, "name", got.Name, "My Copy")
	testutil.Equal(t, "override start", got.TimeRange.StartAt, "2026-02-01T00:00:00Z")
}

// Switching the copy to a non-time-range mode drops the time range entirely.
func TestBuildRequestModeSwitchDropsTimeRange(t *testing.T) {
	permanent := plans.TimeModePermanent
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{TimeMode: &permanent})
	testutil.Equal(t, "time mode", got.TimeMode, permanent)
	if got.TimeRange != nil {
		t.Fatalf("time range should be nil for non-time-range mode, got %+v", got.TimeRange)
	}
}

// A nil override copies the source taxonomy verbatim.
func TestBuildRequestCopiesTaxonomy(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{})
	if got.EnvironmentRef == nil {
		t.Fatal("environmentRef should be copied from the source")
	}
	testutil.Equal(t, "environmentRef", *got.EnvironmentRef, sourceEnvironmentRef)
	testutil.Equal(t, "tagRefs", slices.Equal(got.TagRefs, []string{sourceTagID}), true)
}

// A non-nil override is taken as sent, so an empty override clears rather than
// falling back to the source.
func TestBuildRequestTaxonomyOverrides(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
		TagRefs:        []string{overrideTagA, overrideTagB},
	})
	testutil.Equal(t, "environmentRef", *got.EnvironmentRef, overrideEnvironmentRef)
	testutil.Equal(t, "tagRefs", slices.Equal(got.TagRefs, []string{overrideTagA, overrideTagB}), true)

	cleared := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(""),
		TagRefs:        []string{},
	})
	testutil.Equal(t, "cleared environmentRef", *cleared.EnvironmentRef, "")
	if cleared.TagRefs == nil {
		t.Fatal("cleared tagRefs should be a non-nil empty slice")
	}
	testutil.Equal(t, "cleared tagRefs len", len(cleared.TagRefs), constants.DefaultInitValue)
}

// AvailableName keeps the default copy name when free and otherwise suffixes it until it is.
func TestAvailableName(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     string
	}{
		{"free", nil, defaultCopyName},
		{"taken once", []string{defaultCopyName}, secondCopyName},
		{"taken twice", []string{defaultCopyName, secondCopyName}, "Copy of prod-guard (3)"},
		{"case insensitive clash", []string{"copy of PROD-GUARD"}, secondCopyName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			existing := make([]plans.ProtectionPlan, 0, len(c.existing))
			for i, name := range c.existing {
				existing = append(existing, plans.ProtectionPlan{ID: strconv.Itoa(i), Name: name})
			}
			got, err := duplicate.AvailableName(defaultCopyName, existing)
			testutil.Equal(t, "err", err, nil)
			testutil.Equal(t, "name", got, c.want)
		})
	}
}

// Every candidate being taken is an error rather than an endless search.
func TestAvailableNameExhausted(t *testing.T) {
	existing := []plans.ProtectionPlan{{ID: "0", Name: defaultCopyName}}
	for i := constants.TwoValue; i < takenSuffixCeiling; i++ {
		existing = append(existing, plans.ProtectionPlan{
			ID:   strconv.Itoa(i),
			Name: fmt.Sprintf("Copy of prod-guard (%d)", i),
		})
	}
	if _, err := duplicate.AvailableName(defaultCopyName, existing); err == nil {
		t.Fatal("exhausted suffixes should fail")
	}
}

// UsesDefaultName is true only when the caller left the copy unnamed.
func TestUsesDefaultName(t *testing.T) {
	testutil.Equal(t, "nil", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{}), true)
	testutil.Equal(t, "empty", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{Name: strptr("")}), true)
	testutil.Equal(t, "named", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{Name: strptr("x")}), false)
}

func requiredSource() *plans.ProtectionPlan {
	source := sourcePlan()
	source.ApprovalMode = plans.ApprovalModeRequired
	source.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStateApproved, RequestedBy: requesterID}
	return source
}

func approvalModeOf(t *testing.T, got *planseps.PrepareProtectionPlanRequest) string {
	t.Helper()
	if got.ApprovalMode == nil {
		t.Fatal("approvalMode should be copied from the source")
	}
	return *got.ApprovalMode
}

// A copy with no overrides keeps the source execution mode.
func TestBuildRequestCopiesApprovalModeWhenNothingOverridden(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{})
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeRequired)
}

// Moving the copy to another environment leaves the mode nil so Prepare re-derives it.
func TestBuildRequestDropsApprovalModeWhenEnvironmentChanged(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
	})
	if got.ApprovalMode != nil {
		t.Fatalf("approvalMode = %q, want nil when the environment changed", *got.ApprovalMode)
	}
}

// The UI resends environmentRef whenever tags are touched; an unchanged value is not a change.
func TestBuildRequestKeepsApprovalModeWhenEnvironmentResent(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(sourceEnvironmentRef),
		TagRefs:        []string{overrideTagA},
	})
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeRequired)
}

// An explicit approvalMode override wins even when the environment changes too.
func TestBuildRequestApprovalModeOverrideWins(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
		ApprovalMode:   strptr(plans.ApprovalModeAutomatic),
	})
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeAutomatic)
}

// Approval state never crosses into a copy: the Prepare-shaped DTO has no field for it,
// and a legacy source without a mode yields nil so Prepare derives one.
func TestBuildRequestNeverCarriesApprovalState(t *testing.T) {
	if _, has := reflect.TypeOf(planseps.PrepareProtectionPlanRequest{}).FieldByName("Approval"); has {
		t.Fatal("PrepareProtectionPlanRequest must not carry approval state")
	}
	legacy := sourcePlan()
	legacy.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStatePending, RequestedBy: requesterID}
	got := duplicate.BuildRequest(legacy, planseps.DuplicateProtectionPlanRequest{})
	if got.ApprovalMode != nil {
		t.Fatalf("approvalMode = %q, want nil for a source without a mode", *got.ApprovalMode)
	}
}

// The copy inherits the source exclusions by value, never the source pointer.
func TestBuildRequestCopiesExclusions(t *testing.T) {
	source := sourcePlan()
	source.Scope = plans.ProtectionPlanScope{
		Type:            plans.ScopeTypeApplications,
		ApplicationRefs: []string{"app-1"},
		Exclusions: &plans.ProtectionPlanScopeExclusions{
			Kinds:     []string{"ConfigMap"},
			Resources: []plans.ProtectionPlanExcludedResource{{Kind: "Deployment", Name: "wa1", Namespace: "prod"}},
		},
	}
	got := duplicate.BuildRequest(source, planseps.DuplicateProtectionPlanRequest{})
	testutil.Equal(t, "exclusions", plans.ExclusionsEqual(got.Scope.Exclusions, source.Scope.Exclusions), true)
	if got.Scope.Exclusions == source.Scope.Exclusions {
		t.Fatal("exclusions must be copied, not share the source pointer")
	}

	none := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{})
	if none.Scope.Exclusions != nil {
		t.Fatalf("exclusions = %+v, want nil for a source without exclusions", none.Scope.Exclusions)
	}
}
