package planduplicate

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/duplicate"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

func strptr(s string) *string { return &s }

func sourcePlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		Name:            "prod-guard",
		Severity:        "high",
		Priority:        2,
		Mode:            plans.ModeEnforce,
		TimeMode:        plans.TimeModeTimeRange,
		Scope:           plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}},
		Policies:        []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}},
		TimeRange:       &plans.ProtectionPlanTimeRange{StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z"},
		ParticipantsIDs: []string{"u1"},
	}
}

// With no overrides the copy is named "Copy of <source>", carries the source
// scope and policies, and reuses the source time range.
func TestBuildRequestDefaults(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{})
	testutil.Equal(t, "name", got.Name, "Copy of prod-guard")
	testutil.Equal(t, "scope type", got.Scope.Type, plans.ScopeTypeNamespaces)
	testutil.Equal(t, "policies", len(got.Policies), 1)
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
