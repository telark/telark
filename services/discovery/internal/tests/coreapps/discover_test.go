package coreapps

import (
	"context"
	"testing"

	"github.com/telark/discovery/internal/core/applications/core"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/tests/testutil"
)

// With every input served by the informer manifest cache the enrichment
// fields come out populated without any kind fetcher running: there is no
// Kubernetes client here, so a fetcher would have returned nothing.
func TestDiscoverInputsSkipsFetchersOnCacheHit(t *testing.T) {
	hits := 0
	core.GetManifestFromCache = func(kind, name, ns string) (map[string]any, bool) {
		hits++
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata": map[string]any{
				"name":              name,
				"namespace":         ns,
				"creationTimestamp": "2026-09-01T10:00:00Z",
				"annotations":       map[string]any{"telark.io/last-modified-by": "alice"},
			},
			"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
				"containers": []any{map[string]any{
					"name":  "web",
					"image": "nginx:1.27",
					"ports": []any{map[string]any{"containerPort": int64(8080)}},
				}},
			}}},
		}, true
	}
	t.Cleanup(func() { core.GetManifestFromCache = nil })

	out := core.DiscoverInputsWithK8s(context.Background(), []derivation.ResourceInput{
		{Namespace: "prod", Kind: appshared.KindDeployment, Name: "web"},
	})

	testutil.Equal(t, "cache hits", hits, 1)
	testutil.Equal(t, "createdAt set", out[0].CreatedAt.IsZero(), false)
	testutil.Equal(t, "lastModifiedBy", out[0].LastModifiedBy, "alice")
	testutil.Equal(t, "images", len(out[0].Images), 1)
	testutil.Equal(t, "ports", len(out[0].Ports), 1)
}
