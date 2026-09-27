package planpatches

import (
	"reflect"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	patchTime     = "now"
	cancelUser    = "user-1"
	excludedKind  = "ConfigMap"
	excludedName  = "wa1"
	excludedSpace = "prod"
)

func field(t *testing.T, patch map[string]any, name string) string {
	t.Helper()
	value, ok := patch[name].(string)
	if !ok {
		t.Fatalf("%s = %v, want a string", name, patch[name])
	}
	return value
}

// The typed patch drops an empty renderedPolicies through omitempty, so every terminal
// transition must send the cleared list as a raw field.
func TestTerminalPatchesClearRenderedPolicies(t *testing.T) {
	cases := []struct {
		name      string
		patch     map[string]any
		wantPhase string
	}{
		{"cancel", protection.BuildCancelPatch(cancelUser, "", patchTime), plans.PhaseCanceled},
		{"terminate", protection.BuildTerminatePatch(patchTime), plans.PhaseTerminated},
		{"failed", protection.BuildFailedPatch("render failed", patchTime), plans.PhaseFailed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "phase", field(t, c.patch, protection.FieldPhase), c.wantPhase)
			testutil.Equal(t, "health", field(t, c.patch, protection.FieldHealth), plans.HealthUnknown)
			rendered, ok := c.patch[protection.FieldRenderedPolicies].([]string)
			testutil.Equal(t, "rendered policies typed", ok, true)
			testutil.Equal(t, "rendered policies empty", len(rendered), 0)
			if rendered == nil {
				t.Fatal("rendered policies must be an empty slice, not nil")
			}
		})
	}
}

// A cancel without a caller-supplied reason still records why the plan stopped.
func TestBuildCancelPatchDefaultReason(t *testing.T) {
	defaulted := protection.BuildCancelPatch(cancelUser, "", patchTime)
	testutil.Equal(t, "default reason", field(t, defaulted, protection.FieldReason), protection.ReasonCanceledByUser)

	given := protection.BuildCancelPatch(cancelUser, "not needed", patchTime)
	testutil.Equal(t, "given reason", field(t, given, protection.FieldReason), "not needed")
}

// Merge patch deletes a key set to null, so absent and empty exclusions both clear the stored object.
func TestExclusionsPatchValueNullWhenEmpty(t *testing.T) {
	for name, excl := range map[string]*plans.ProtectionPlanScopeExclusions{"nil": nil, "empty": {}} {
		if got := protection.ExclusionsPatchValue(excl); got != nil {
			t.Fatalf("%s exclusions = %#v, want nil", name, got)
		}
	}
}

// Merge patch merges objects key by key and replaces arrays, so both arrays are always sent.
func TestExclusionsPatchValueAlwaysBothKeys(t *testing.T) {
	resource := plans.ProtectionPlanExcludedResource{Kind: excludedKind, Name: excludedName, Namespace: excludedSpace}
	cases := []struct {
		name string
		in   plans.ProtectionPlanScopeExclusions
		want map[string]any
	}{
		{"kinds only", plans.ProtectionPlanScopeExclusions{Kinds: []string{excludedKind}}, map[string]any{
			protection.FieldExclusionKinds:     []any{excludedKind},
			protection.FieldExclusionResources: []any{},
		}},
		{"resources only", plans.ProtectionPlanScopeExclusions{Resources: []plans.ProtectionPlanExcludedResource{resource}}, map[string]any{
			protection.FieldExclusionKinds: []any{},
			protection.FieldExclusionResources: []any{map[string]any{
				protection.FieldExclusionKind:      excludedKind,
				protection.FieldExclusionName:      excludedName,
				protection.FieldExclusionNamespace: excludedSpace,
			}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := protection.ExclusionsPatchValue(&c.in); !reflect.DeepEqual(got, any(c.want)) {
				t.Fatalf("patch value = %#v, want %#v", got, c.want)
			}
		})
	}
}
