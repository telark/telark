package manifest

import (
	"testing"

	"github.com/telark/kcore/manifest"
)

const (
	fieldMetadata        = "metadata"
	fieldOwnerReferences = "ownerReferences"
	fieldFinalizers      = "finalizers"
	fieldName            = "name"
	fieldKind            = "kind"
	objectName           = "web"
	finalizerName        = "example.com/keep"
)

func withOwner(kind string) map[string]any {
	return map[string]any{
		fieldKind: kind,
		fieldMetadata: map[string]any{
			fieldName:            objectName,
			fieldFinalizers:      []any{finalizerName},
			fieldOwnerReferences: []any{map[string]any{"uid": "stale-owner-uid", fieldKind: "ReplicaSet"}},
		},
	}
}

func TestCleanManifestForApplyDropsOwnerReferences(t *testing.T) {
	for _, kind := range []string{"Deployment", "ConfigMap", "Job", "Secret"} {
		t.Run(kind, func(t *testing.T) {
			m := withOwner(kind)
			manifest.CleanManifestForApply(m)

			meta, ok := m[fieldMetadata].(map[string]any)
			if !ok {
				t.Fatalf("metadata missing after cleaning: %v", m)
			}
			if _, present := meta[fieldOwnerReferences]; present {
				t.Fatalf("ownerReferences survived cleaning: %v", meta)
			}
			if meta[fieldName] != objectName {
				t.Fatalf("name lost: %v", meta)
			}
			if _, present := meta[fieldFinalizers]; !present {
				t.Fatalf("finalizers of a %s must be kept: %v", kind, meta)
			}
		})
	}
}
