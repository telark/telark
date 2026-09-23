package policies

import (
	"testing"

	kyvernovalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/telark/data/plans"
	datapolicies "github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers every renderer
	"github.com/telark/discovery/internal/constants"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	acceptNamespace = "prod"
	acceptApp       = "wa1"
)

// Params for the two templates that take them; the rest render without any.
var acceptParams = map[string]map[string]any{
	"block-image-types": {"imagePatterns": []string{"*nginx*"}},
	"block-image-tags":  {"tags": []string{"latest"}},
}

func acceptScopes() map[string]datapolicies.ScopeSpec {
	kinds := []string{"Deployment", "StatefulSet", "ConfigMap", "Secret", "CronJob"}
	refs := make([]datapolicies.ApplicationResourceRef, constants.DefaultInitValue, len(kinds))
	for _, kind := range kinds {
		refs = append(refs, datapolicies.ApplicationResourceRef{
			Kind: kind, Name: acceptApp, Namespace: acceptNamespace,
		})
	}
	return map[string]datapolicies.ScopeSpec{
		"namespaces": {Namespace: acceptNamespace},
		"applications": {
			Namespace:      acceptNamespace,
			ApplicationIDs: []string{acceptApp},
			AppResources:   refs,
			VolumeClaims:   []string{"pvc-a", "data-wa1-*"},
		},
	}
}

// Cluster-scoped kinds a namespaced Policy may not reference. Kyverno derives this set from
// live discovery and hands it to the same check we run below; in mock mode it hands over an
// EMPTY set, so the check runs and can never fire. Supplying it here is what closes that.
var clusterScopedKinds = sets.New(
	"ClusterPolicy",
	"Namespace",
	"Node",
	"PersistentVolume",
	"StorageClass",
	"ClusterRole",
	"ClusterRoleBinding",
	"CustomResourceDefinition",
	"PriorityClass",
	"ValidatingWebhookConfiguration",
	"MutatingWebhookConfiguration",
)

// Rendering assertions only prove the shape we meant to emit; they cannot say whether Kyverno
// would accept it. A rule mixing `*` with another kind, and later a `ClusterPolicy` kind inside
// a namespaced Policy, both rendered exactly as intended and were rejected at admission with a
// 503. Every rendered policy goes through Kyverno's own validation here, in both the passes the
// webhook runs. No cluster needed.
func TestKyvernoAcceptsEveryRenderedPolicy(t *testing.T) {
	for _, tpl := range plans.Templates {
		for scopeName, scope := range acceptScopes() {
			t.Run(tpl.ID+"/"+scopeName, func(t *testing.T) {
				renderer, ok := datapolicies.GetRenderer(tpl.ID)
				if !ok {
					t.Fatalf("no renderer registered for %s", tpl.ID)
				}
				meta := datapolicies.RenderMeta{
					PlanID:   "pp-abc-1234-5678",
					PlanName: "freeze",
					Mode:     plans.ModeEnforce,
				}
				pol, err := renderer.Render(meta, scope, acceptParams[tpl.ID])
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				if pol == nil {
					t.Fatal("template rendered no policy")
				}
				if !pol.IsNamespaced() || pol.GetNamespace() == "" {
					t.Fatal("guard must validate the artifact we deploy: a namespaced Policy")
				}
				if _, err := kyvernovalidation.Validate(pol, nil, nil, true, "", ""); err != nil {
					t.Errorf("Kyverno would reject this policy: %v", err)
				}
				// The second pass, with the cluster-scoped set the webhook derives from
				// discovery and mock mode leaves empty.
				if _, errs := pol.Validate(clusterScopedKinds); len(errs) > 0 {
					t.Errorf("Kyverno would reject this policy: %v", errs.ToAggregate())
				}
			})
		}
	}
}
