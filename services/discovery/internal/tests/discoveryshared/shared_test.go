package shared

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/discovery/shared"
	"github.com/telark/discovery/internal/tests/testutil"
)

// A comma list is split and trimmed; blank or the sentinel "ALL" means "no
// filter" and yields nil.
func TestParseNamespaceList(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"", 0},
		{"ALL", 0},
		{"all", 0},
		{"a, b ,,c", 3},
		{" solo ", 1},
	}
	for _, c := range cases {
		testutil.Equal(t, "len:"+c.raw, len(shared.ParseNamespaceList(c.raw)), c.want)
	}
}

// The primary namespace is the alphabetically-first key; no counts means empty.
func TestPrimaryNamespaceFromCounts(t *testing.T) {
	testutil.Equal(t, "empty", shared.PrimaryNamespaceFromCounts(nil), "")
	testutil.Equal(t, "first", shared.PrimaryNamespaceFromCounts(map[string]int{"z": 1, "a": 2}), "a")
}

// Namespace items are sorted by name and carry their resource counts.
func TestBuildNamespaceItems(t *testing.T) {
	items := shared.BuildNamespaceItems(map[string]int{"z": 2, "a": 1})
	if len(items) != 2 || items[0].Name != "a" || items[1].Name != "z" {
		t.Fatalf("items not sorted by name: %+v", items)
	}
	testutil.Equal(t, "count", items[0].ResourceCount, 1)
}

// Kind counts map onto the typed resource summary.
func TestResourceSummaryFromKindCounts(t *testing.T) {
	sum := shared.ResourceSummaryFromKindCounts(map[string]int{"Deployment": 3, "Service": 1})
	testutil.Equal(t, "deployments", sum.Deployment, 3)
	testutil.Equal(t, "services", sum.Service, 1)
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
	unknown := shared.BuildManaged("", "", "")
	testutil.Equal(t, "unknown.by", unknown.By, shared.ManagedUnknown)
	if unknown.Chart != nil || unknown.Version != nil {
		t.Fatal("empty chart/version must stay nil")
	}
}

// Display names strip the namespace prefix and title-case the remainder.
func TestDisplayNames(t *testing.T) {
	if shared.BuildDisplayName("prod-shop-api", "prod") == "" {
		t.Fatal("display name is empty")
	}
	if shared.ToDisplayName("my-service") == "" {
		t.Fatal("display name is empty")
	}
}

// Group order follows first appearance and de-duplicates.
func TestOrderedGroupNames(t *testing.T) {
	withGroups := []derivation.ResourceWithGroup{
		{Group: "b"}, {Group: "a"}, {Group: "b"},
	}
	got := shared.OrderedGroupNames(withGroups)
	if len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("order = %v, want [b a]", got)
	}
}

// CoalesceStrings never returns nil; ToDerivationInputs on nil is a no-op.
func TestCoalesceAndDerivationInputs(t *testing.T) {
	if got := shared.CoalesceStrings(nil); got == nil {
		t.Fatal("nil coalesced to nil")
	}
	testutil.Equal(t, "non-nil pass-through", len(shared.CoalesceStrings([]string{"x"})), 1)
	testutil.Equal(t, "nil refs", len(shared.ToDerivationInputs(nil)), 0)
}
