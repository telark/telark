package manifest

import (
	"encoding/json"
	"testing"

	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/kcore/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	configMapKind        = "ConfigMap"
	unmanagedKind        = "Widget"
	fetchNamespace       = "apps"
	missingObjectName    = "absent"
	fieldResourceVersion = "resourceVersion"
	staleResourceVersion = "42"
)

func TestGetRawManifest(t *testing.T) {
	// GetRawManifest keeps the first dynamic client it gets for the whole process,
	// so every case and every -count run reads the same seeded object.
	k8sclient.SetDynamicClient(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		fieldKind:    configMapKind,
		fieldMetadata: map[string]any{
			fieldName: objectName, "namespace": fetchNamespace, fieldResourceVersion: staleResourceVersion,
		},
	}}))
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })

	cases := []struct {
		name, kind, object    string
		wantManifest, wantErr bool
	}{
		{name: "stored object", kind: configMapKind, object: objectName, wantManifest: true},
		{name: "missing object", kind: configMapKind, object: missingObjectName, wantErr: true},
		{name: "unmanaged kind", kind: unmanagedKind, object: objectName},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := manifest.GetRawManifest(t.Context(), tc.kind, tc.object, fetchNamespace)
			if (err != nil) != tc.wantErr || (raw != nil) != tc.wantManifest {
				t.Fatalf("GetRawManifest(%s, %s) = %s, %v", tc.kind, tc.object, raw, err)
			}
			if !tc.wantManifest {
				return
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("manifest is not JSON: %v", err)
			}
			meta, ok := got[fieldMetadata].(map[string]any)
			if _, present := meta[fieldResourceVersion]; !ok || present || meta[fieldName] != objectName {
				t.Fatalf("manifest not cleaned for apply: %v", meta)
			}
		})
	}
}
