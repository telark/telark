package policies

import (
	"testing"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const combinationCount = 4

// Combinations is the cartesian product of policies and targets; an empty side
// yields no pairs.
func TestCombinations(t *testing.T) {
	plist := []plans.ProtectionPlanPolicy{{TemplateID: "t1"}, {TemplateID: "t2"}}
	targets := []string{"ns1", "ns2"}
	testutil.Equal(t, "product", len(policies.Combinations(plist, targets)), combinationCount)
	testutil.Equal(t, "no policies", len(policies.Combinations(nil, targets)), constants.DefaultInitValue)
	testutil.Equal(t, "no targets", len(policies.Combinations(plist, nil)), constants.DefaultInitValue)
}

// UnionRenderedNames merges two name lists, dropping duplicates while preserving
// first-seen order.
func TestUnionRenderedNames(t *testing.T) {
	got := policies.UnionRenderedNames([]string{"a", "b"}, []string{"b", "c"})
	if len(got) != constants.ThreeValue || got[constants.DefaultInitValue] != "a" || got[constants.TwoValue] != "c" {
		t.Fatalf("union = %v, want [a b c]", got)
	}
	testutil.Equal(t, "empty", len(policies.UnionRenderedNames(nil, nil)), constants.DefaultInitValue)
}
