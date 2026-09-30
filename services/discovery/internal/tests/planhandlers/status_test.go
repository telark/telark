package planhandlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/data/plans"
	dpolicies "github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers the renderers the check compares against
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	handlers "github.com/telark/discovery/internal/handlers/plans/protection"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// D9: the status drift carried only missing and unexpected, so a hand-edited or mis-moded policy,
// and one the plan never listed, read as healthy in the API.
func TestStatusReportsEveryDriftKind(t *testing.T) {
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-5678",
		Phase:    plans.PhaseActive,
		Mode:     plans.ModeEnforce,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}, {TemplateID: "block-delete"}},
	}
	rendered, err := dpolicies.Render(plan, nil, nil)
	testutil.Equal(t, "render", err, nil)
	edited, added := rendered[constants.DefaultInitValue], rendered[constants.DefaultAddValue].Name
	plan.RenderedPolicies = []string{edited.Name}
	edited.Spec.ValidationFailureAction = dpolicies.FailureAction(plans.ModeAudit)
	edited.Spec.Rules[constants.DefaultInitValue].Validation.Message = "edited by hand"
	live, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&edited)
	testutil.Equal(t, "to unstructured", err, nil)
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{protpolicies.KyvernoPolicyGVR: "PolicyList"},
		&unstructured.Unstructured{Object: live},
	)
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, marshalErr := json.Marshal(map[string]any{"status": http.StatusOK, "data": plan})
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, marshalErr
	})
	handlers.InitService(protection.NewService(nil, nil, clients.NewProtectionPlanClient(), dyn, nil, nil, nil, nil))
	t.Cleanup(func() {
		http.DefaultTransport = prev
		handlers.InitService(nil)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protectionplans/"+plan.ID+"/status", nil)
	handlers.Status(rec, mux.SetURLVars(req, map[string]string{constants.IDPathParam: plan.ID}))

	testutil.Equal(t, "status", rec.Code, http.StatusOK)
	var got struct {
		Data planseps.ProtectionPlanStatusResponse `json:"data"`
	}
	testutil.Equal(t, "decode", json.Unmarshal(rec.Body.Bytes(), &got), nil)
	testutil.Equal(t, "mismatched", slices.Equal(got.Data.Drift.Mismatched, []string{edited.Name}), true)
	testutil.Equal(t, "stale", slices.Equal(got.Data.Drift.Stale, []string{edited.Name}), true)
	testutil.Equal(t, "added", slices.Equal(got.Data.Drift.Added, []string{added}), true)
}
