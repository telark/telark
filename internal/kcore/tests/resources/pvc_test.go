package resources

import (
	"testing"

	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/kcore/resources/core"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func claim(name string, status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		FieldAPIVersion: ClaimAPIVersion,
		FieldKind:       ClaimKind,
		FieldMetadata:   map[string]any{FieldName: name, FieldNamespace: TestClaimNamespace},
		FieldStatus:     status,
	}}
}

func TestPersistentVolumeClaimCapacityBytes(t *testing.T) {
	cases := []struct {
		name    string
		claim   string
		status  map[string]any
		want    int64
		wantErr bool
	}{
		{
			name: "bound claim", claim: TestClaimName, want: TestClaimCapacityBytes,
			status: map[string]any{FieldCapacity: map[string]any{FieldStorage: TestClaimCapacity}},
		},
		{name: "no capacity yet", claim: TestClaimName, status: map[string]any{}, wantErr: true},
		{
			name: "unreadable capacity", claim: TestClaimName, wantErr: true,
			status: map[string]any{FieldCapacity: map[string]any{FieldStorage: TestClaimBadCapacity}},
		},
		{name: "missing claim", claim: TestClaimMissing, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k8sclient.SetDynamicClient(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), claim(TestClaimName, tc.status)))
			t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
			got, err := core.GetPersistentVolumeClaimCapacityBytes(TestClaimNamespace, tc.claim)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf(ExpectedClaimCapacity, tc.claim, got, err, tc.want, tc.wantErr)
			}
		})
	}
}
