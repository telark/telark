package planhandlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	runStartedAt = "2026-09-20T10:00:00Z"
	runRequester = "test@example.com"
)

type ledgerStore struct{ ledger json.RawMessage }

func (ledgerStore) Create(reportseps.CreatePlanReportRequest) (*reportseps.ReportMeta, error) {
	return &reportseps.ReportMeta{}, nil
}
func (ledgerStore) PutLedger(string, json.RawMessage) error     { return nil }
func (s ledgerStore) GetLedger(string) (json.RawMessage, error) { return s.ledger, nil }

// window=run lists the plan's run from the reports' ledger, so a finished plan, whose policies
// and live Events are gone, still shows who tried what; the response says when the run started.
func TestViolationsRunWindowServesTheLedger(t *testing.T) {
	started, ended := runStartedAt, "2026-09-20T12:00:00Z"
	plan := &plans.ProtectionPlan{ID: "pp-abc-1234-5678", Phase: plans.PhaseTerminated, StartedAt: &started, TerminatedAt: &ended,
		Scope: plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}}}
	ledger, err := json.Marshal(reports.PlanReportLedger{PlanID: plan.ID, Run: "20260920T100000Z", Truncated: true,
		Violations: []planseps.ProtectionPlanViolation{{Timestamp: "2026-09-20T11:00:00Z", Result: "fail", RequestedBy: runRequester}}})
	testutil.Equal(t, "ledger", err, nil)
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{{Version: "v1", Resource: "events"}: "EventList"})
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		body, marshalErr := json.Marshal(map[string]any{"status": http.StatusOK, "data": plan})
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, marshalErr
	})
	gen := reports.NewGenerator(ledgerStore{ledger: ledger}, dyn, nil, nil, nil, constants.DefaultReportMaxViolations, time.Minute)
	handlers.InitService(protection.NewService(nil, nil, clients.NewProtectionPlanClient(), dyn, nil, gen, nil, nil))
	t.Cleanup(func() {
		http.DefaultTransport = prev
		handlers.InitService(nil)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/protectionplans/"+plan.ID+"/violations?window=run", nil)
	handlers.Violations(rec, mux.SetURLVars(req, map[string]string{constants.IDPathParam: plan.ID}))

	testutil.Equal(t, "status", rec.Code, http.StatusOK)
	var got struct {
		Data planseps.ProtectionPlanViolationsResponse `json:"data"`
	}
	testutil.Equal(t, "decode", json.Unmarshal(rec.Body.Bytes(), &got), nil)
	testutil.Equal(t, "rows", got.Data.Total, constants.DefaultAddValue)
	testutil.Equal(t, "requester", got.Data.Violations[constants.DefaultInitValue].RequestedBy, runRequester)
	testutil.Equal(t, "since", got.Data.Since, runStartedAt)
	testutil.Equal(t, "truncated", got.Data.Truncated, true)
}
