package derivation

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/tests/testutil"
)

func inputs() []derivation.ResourceInput {
	appLabel := map[string]string{"app.kubernetes.io/name": "shop"}
	return []derivation.ResourceInput{
		{Namespace: "prod", Kind: "Deployment", Name: "shop-api", Labels: appLabel},
		{Namespace: "prod", Kind: "Service", Name: "shop-svc", Labels: appLabel},
		{Namespace: "prod", Kind: "ConfigMap", Name: "shop-cfg"},
		{Namespace: "prod", Kind: "ConfigMap", Name: "kube-root-ca.crt"},
	}
}

// FilterNoise drops the well-known cluster-injected resources but keeps the rest.
func TestFilterNoise(t *testing.T) {
	kept := derivation.FilterNoise(inputs())
	testutil.Equal(t, "kept", len(kept), 3)
	for _, r := range kept {
		if r.Name == "kube-root-ca.crt" {
			t.Fatal("noise resource survived the filter")
		}
	}
}

// A labelled workload anchors a group, and unlabelled resources whose name
// contains the app key are attached to it — one namespace, one group named for
// the app label.
func TestGroupByWorkloadAnchor(t *testing.T) {
	groups := derivation.GroupByWorkloadAnchor(inputs())
	if len(groups) == 0 {
		t.Fatal("labelled workload produced no groups")
	}
	for _, g := range groups {
		testutil.Equal(t, "group", g.Group, "shop")
		testutil.Equal(t, "namespace", g.Namespace, "prod")
	}
}

// DeriveGroups returns the grouped resources plus the ordered list of group names.
func TestDeriveGroups(t *testing.T) {
	withGroups, names := derivation.DeriveGroups(inputs())
	if len(withGroups) == 0 || len(names) == 0 {
		t.Fatalf("derive produced %d resources / %d names", len(withGroups), len(names))
	}
}

// Empty input is a no-op, not a panic.
func TestGroupByWorkloadAnchorEmpty(t *testing.T) {
	if got := derivation.GroupByWorkloadAnchor(nil); got != nil {
		t.Fatalf("nil input produced %d groups", len(got))
	}
}

// The group name comes from the first recognised label key, else the raw name.
func TestGroupNameFromLabels(t *testing.T) {
	testutil.Equal(t, "no labels", derivation.FirstGroupNameFromLabels(nil), "")
	labels := map[string]string{"app": "billing"}
	testutil.Equal(t, "legacy label", derivation.FirstGroupNameFromLabels(labels), "billing")
	testutil.Equal(t, "fallback", derivation.GroupNameOrFallback("raw-name", nil), "raw-name")
	testutil.Equal(t, "label wins", derivation.GroupNameOrFallback("raw-name", labels), "billing")
}
