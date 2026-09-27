package planlifecycle

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/telark/data/plans"
	dpolicies "github.com/telark/data/policies"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/core/plans/protection"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	planID     = "pp-abc-1234-5678"
	policyName = "telark-pp-abc-1234-5678-bc-0"
	planNS     = "prod"
	userID     = "u-1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type nopLogger struct{}

func (nopLogger) Info(string)  {}
func (nopLogger) Error(string) {}

func answer(t *testing.T, status int, data any) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": status, "message": "m", "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
}

func stubExporter(t *testing.T, patchStatus int) {
	t.Helper()
	plan := plans.ProtectionPlan{ID: planID, Phase: plans.PhaseActive, RenderedPolicies: []string{policyName}}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPatch {
			return answer(t, patchStatus, nil), nil
		}
		return answer(t, http.StatusOK, plan), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}

func clusterWithPolicy() *dynamicfake.FakeDynamicClient {
	pol := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1",
		"kind":       "Policy",
		"metadata": map[string]any{
			"name":      policyName,
			"namespace": planNS,
			"labels":    map[string]any{dpolicies.LabelPlanID: planID, dpolicies.LabelManagedBy: dpolicies.ManagedByValue},
		},
	}}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{protpolicies.KyvernoPolicyGVR: "PolicyList"}, pol,
	)
}

// A failed phase patch used to follow the policy cleanup, leaving an "active" plan that enforced
// nothing; the patch now goes first and the policies stay until it lands.
func TestCancelPatchesPhaseBeforeCleanup(t *testing.T) {
	cases := []struct {
		name        string
		patchStatus int
		wantErr     bool
		wantPolicy  bool
	}{
		{"patch refused keeps policies", http.StatusBadRequest, true, true},
		{"patch landed removes policies", http.StatusOK, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubExporter(t, c.patchStatus)
			dyn := clusterWithPolicy()
			applier := protpolicies.NewApplier(dyn, meta.NewDefaultRESTMapper(nil))
			svc := protection.NewService(applier, nil, clients.NewProtectionPlanClient(), dyn, nil, nil, nopLogger{}, nil)

			_, err := svc.Cancel(context.Background(), userID, planID, "")
			testutil.Equal(t, "error", err != nil, c.wantErr)
			_, getErr := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(planNS).Get(context.Background(), policyName, metav1.GetOptions{})
			testutil.Equal(t, "policy present", getErr == nil, c.wantPolicy)
		})
	}
}
