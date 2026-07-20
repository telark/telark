package derivation

import (
	"testing"

	"github.com/telark/discovery/internal/discovery/derivation"
)

// Unlabelled resources are attached to a labelled workload by walking their
// owner-reference chain to the identified root: a Pod owned by a ReplicaSet owned
// by the labelled Deployment all land in the same group.
func TestGroupByOwnerReferenceChain(t *testing.T) {
	appLabel := map[string]string{"app.kubernetes.io/name": "shop"}
	inputs := []derivation.ResourceInput{
		{Namespace: "prod", Kind: "Deployment", Name: "shop-api", Labels: appLabel},
		{
			Namespace: "prod", Kind: "ReplicaSet", Name: "shop-api-rs",
			OwnerReferences: []derivation.OwnerReference{{Kind: "Deployment", Name: "shop-api"}},
		},
		{
			Namespace: "prod", Kind: "Pod", Name: "shop-api-rs-xyz",
			OwnerReferences: []derivation.OwnerReference{{Kind: "ReplicaSet", Name: "shop-api-rs"}},
		},
	}

	groups := derivation.GroupByWorkloadAnchor(inputs)
	if len(groups) == 0 {
		t.Fatal("owner-reference chain produced no groups")
	}
	for _, g := range groups {
		if g.Group != "shop" {
			t.Fatalf("resource %s/%s grouped under %q, want shop", g.Kind, g.Name, g.Group)
		}
	}
}
