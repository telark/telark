package discoveryshared

import (
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/discovery/shared"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	groupB = "b"
	groupA = "a"
)

// A comma list is split and trimmed; blank or the sentinel "ALL" means "no
// filter" and yields nil.
func TestParseNamespaceList(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", constants.DefaultInitValue},
		{"ALL", constants.DefaultInitValue},
		{"all", constants.DefaultInitValue},
		{"a, b ,,c", constants.ThreeValue},
		{" solo ", constants.DefaultAddValue},
	}
	for _, c := range cases {
		testutil.Equal(t, "len:"+c.raw, len(shared.ParseNamespaceList(c.raw)), c.want)
	}
}

// Namespace items are sorted by name and carry their resource counts.
func TestBuildNamespaceItems(t *testing.T) {
	items := shared.BuildNamespaceItems(map[string]int{"z": 2, groupA: 1})
	if len(items) != constants.TwoValue || items[constants.DefaultInitValue].Name != groupA || items[constants.DefaultAddValue].Name != "z" {
		t.Fatalf("items not sorted by name: %+v", items)
	}
	testutil.Equal(t, "count", items[0].ResourceCount, constants.DefaultAddValue)
}

// Kind counts map onto the typed resource summary.
func TestResourceSummaryFromKindCounts(t *testing.T) {
	sum := shared.ResourceSummaryFromKindCounts(map[string]int{"Deployment": 3, "Service": 1})
	testutil.Equal(t, "deployments", sum.Deployment, constants.ThreeValue)
	testutil.Equal(t, "services", sum.Service, constants.DefaultAddValue)
}

// Managed source normalises to helm, manual, or unknown; chart and version
// become pointers only when present.
func TestBuildManaged(t *testing.T) {
	helm := shared.BuildManaged(shared.ManagedHelm, "shop-1.0", "1.0")
	testutil.Equal(t, "helm.by", helm.By, shared.ManagedHelm)
	if helm.Chart == nil || helm.Version == nil {
		t.Fatal("chart/version should be set")
	}
	testutil.Equal(t, "manual.by", shared.BuildManaged("argocd", "", "").By, shared.ManagedManual)
	unknown := shared.BuildManaged("", constants.EmptyString, constants.EmptyString)
	testutil.Equal(t, "unknown.by", unknown.By, shared.ManagedUnknown)
	if unknown.Chart != nil || unknown.Version != nil {
		t.Fatal("empty chart/version must stay nil")
	}
}

// Display names keep the namespace prefix so "demo3-report" in ns demo3 does
// not collapse to "Report".
func TestDisplayNames(t *testing.T) {
	testutil.Equal(t, "display name", shared.BuildDisplayName("demo3-report"), "Demo3 Report")
	if shared.BuildDisplayName("my-service") == constants.EmptyString {
		t.Fatal("display name is empty")
	}
}

// Group order follows first appearance and de-duplicates.
func TestOrderedGroupNames(t *testing.T) {
	withGroups := []derivation.ResourceWithGroup{
		{Group: groupB}, {Group: groupA}, {Group: groupB},
	}
	got := shared.OrderedGroupNames(withGroups)
	if len(got) != constants.TwoValue || got[constants.DefaultInitValue] != groupB || got[constants.DefaultAddValue] != groupA {
		t.Fatalf("order = %v, want [b a]", got)
	}
}

// CoalesceStrings never returns nil; ToDerivationInputs on nil is a no-op.
func TestCoalesceAndDerivationInputs(t *testing.T) {
	if got := shared.CoalesceStrings(nil); got == nil {
		t.Fatal("nil coalesced to nil")
	}
	testutil.Equal(t, "non-nil pass-through", len(shared.CoalesceStrings([]string{"x"})), constants.DefaultAddValue)
	testutil.Equal(t, "nil refs", len(shared.ToDerivationInputs(nil)), constants.DefaultInitValue)
}
