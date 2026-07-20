package policies

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
)

// Combinations is the cartesian product of policies and targets; an empty side
// yields no pairs.
func TestCombinations(t *testing.T) {
	plist := []plans.ProtectionPlanPolicy{{TemplateID: "t1"}, {TemplateID: "t2"}}
	targets := []string{"ns1", "ns2"}
	testutil.Equal(t, "product", len(policies.Combinations(plist, targets)), 4)
	testutil.Equal(t, "no policies", len(policies.Combinations(nil, targets)), 0)
	testutil.Equal(t, "no targets", len(policies.Combinations(plist, nil)), 0)
}

// UnionRenderedNames merges two name lists, dropping duplicates while preserving
// first-seen order.
func TestUnionRenderedNames(t *testing.T) {
	got := policies.UnionRenderedNames([]string{"a", "b"}, []string{"b", "c"})
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("union = %v, want [a b c]", got)
	}
	testutil.Equal(t, "empty", len(policies.UnionRenderedNames(nil, nil)), 0)
}
