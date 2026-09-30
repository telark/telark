package derivation

import (
	"testing"

	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	prodNamespace = "prod"
	rawName       = "raw-name"
	billingGroup  = "billing"
)

func inputs() []derivation.ResourceInput {
	appLabel := map[string]string{"app.kubernetes.io/name": "shop"}
	return []derivation.ResourceInput{
		{Namespace: prodNamespace, Kind: "Deployment", Name: "shop-api", Labels: appLabel},
		{Namespace: prodNamespace, Kind: "Service", Name: "shop-svc", Labels: appLabel},
		{Namespace: prodNamespace, Kind: "ConfigMap", Name: "shop-cfg"},
		{Namespace: prodNamespace, Kind: "ConfigMap", Name: "kube-root-ca.crt"},
	}
}

// FilterNoise drops the well-known cluster-injected resources but keeps the rest.
func TestFilterNoise(t *testing.T) {
	kept := derivation.FilterNoise(inputs())
	testutil.Equal(t, "kept", len(kept), constants.ThreeValue)
	for _, r := range kept {
		if r.Name == "kube-root-ca.crt" {
			t.Fatal("noise resource survived the filter")
		}
	}
}

// A labeled workload anchors a group, and unlabeled resources whose name
// contains the app key are attached to it — one namespace, one group named for
// the app label.
func TestGroupByWorkloadAnchor(t *testing.T) {
	groups := derivation.GroupByWorkloadAnchor(inputs())
	if len(groups) == constants.DefaultInitValue {
		t.Fatal("labeled workload produced no groups")
	}
	for _, g := range groups {
		testutil.Equal(t, "group", g.Group, "shop")
		testutil.Equal(t, "namespace", g.Namespace, prodNamespace)
	}
}

// DeriveGroups returns the grouped resources plus the ordered list of group names.
func TestDeriveGroups(t *testing.T) {
	withGroups, names := derivation.DeriveGroups(inputs())
	if len(withGroups) == constants.DefaultInitValue || len(names) == constants.DefaultInitValue {
		t.Fatalf("derive produced %d resources / %d names", len(withGroups), len(names))
	}
}

// Empty input is a no-op, not a panic.
func TestGroupByWorkloadAnchorEmpty(t *testing.T) {
	if got := derivation.GroupByWorkloadAnchor(nil); got != nil {
		t.Fatalf("nil input produced %d groups", len(got))
	}
}

// The group name comes from the first recognized label key, else the raw name.
func TestGroupNameFromLabels(t *testing.T) {
	testutil.Equal(t, "no labels", derivation.FirstGroupNameFromLabels(nil), "")
	labels := map[string]string{"app": billingGroup}
	testutil.Equal(t, "legacy label", derivation.FirstGroupNameFromLabels(labels), billingGroup)
	testutil.Equal(t, "fallback", derivation.GroupNameOrFallback(rawName, nil), rawName)
	testutil.Equal(t, "label wins", derivation.GroupNameOrFallback(rawName, labels), billingGroup)
}
