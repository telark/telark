package reports

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	globalshared "github.com/telark/telark/internal/data/shared"
	reportsclient "github.com/telark/telark/internal/rest/clients/reports"
	"github.com/telark/telark/internal/rest/clients/shared"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
)

const (
	testServiceToken = "test-service-token"
	planID           = "p1"
	reportID         = "r1"
	fileHTML         = "<html></html>"
	okBody           = `{"status":200,"operation":"Success"}`
	notFoundBody     = `{"status":404,"operation":"Failure"}`
	createPath       = "/api/v1/internal/reports"
	ledgerPath       = "/api/v1/internal/protectionplans/p1/ledger"
	ledgerBody       = `{"planId":"p1","run":"r","checkpoints":[{"at":"t","health":"healthy","violationsSeen":1}],"violations":[]}`
)

// The token is read once per process, so it must be set before any test in
// this binary sends its first request.
func init() {
	if err := os.Setenv(dataconstants.EnvServiceToken, testServiceToken); err != nil {
		panic(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type captured struct {
	method, path, query, token string
	body                       []byte
}

func serve(t *testing.T, status int, reply string) (*reportsclient.Client, *captured) {
	t.Helper()
	got := &captured{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*got = captured{
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
			token: r.Header.Get(dataconstants.HeaderServiceToken), body: body,
		}
		w.WriteHeader(status)
		if _, err := w.Write([]byte(reply)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := reportsclient.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
	return client, got
}

func TestCreatePostsEnvelopeWithServiceToken(t *testing.T) {
	client, got := serve(t, http.StatusOK, okBody)

	resp := client.CreatePlanReport(reportseps.CreatePlanReportRequest{
		Meta:  reportseps.ReportMeta{ID: reportID, PlanID: planID, Trigger: reportseps.TriggerManual},
		Files: map[string]string{reportseps.FormatHTML: fileHTML},
	})
	if resp == nil || resp.Status != globalshared.StatusOK {
		t.Fatalf("expected status %d, got %+v", globalshared.StatusOK, resp)
	}
	if got.method != http.MethodPost || got.path != createPath {
		t.Fatalf("expected POST %s, got %s %s", createPath, got.method, got.path)
	}
	if got.token != testServiceToken {
		t.Fatalf("expected %s %q, got %q", dataconstants.HeaderServiceToken, testServiceToken, got.token)
	}
	var wire reportseps.CreatePlanReportRequest
	if err := json.Unmarshal(got.body, &wire); err != nil {
		t.Fatalf("body is not a CreatePlanReportRequest: %v (%s)", err, got.body)
	}
	if wire.Meta.PlanID != planID || wire.Meta.ID != reportID || wire.Files[reportseps.FormatHTML] != fileHTML {
		t.Fatalf("meta/files did not round-trip: %+v", wire)
	}
}

func TestGetLedgerMapsNotFound(t *testing.T) {
	client, got := serve(t, http.StatusNotFound, notFoundBody)

	ledger, err := client.GetLedger(planID)
	if !errors.Is(err, shared.ErrNotFound) {
		t.Fatalf("expected shared.ErrNotFound, got ledger=%s err=%v", ledger, err)
	}
	if got.method != http.MethodGet || got.path != ledgerPath {
		t.Fatalf("unexpected request %s %s", got.method, got.path)
	}
}

func TestPutLedgerPutsRawBytesVerbatim(t *testing.T) {
	client, got := serve(t, http.StatusOK, okBody)

	resp := client.PutLedger(planID, json.RawMessage(ledgerBody))
	if resp == nil || resp.Status != globalshared.StatusOK {
		t.Fatalf("expected status %d, got %+v", globalshared.StatusOK, resp)
	}
	if got.method != http.MethodPut || got.path != ledgerPath {
		t.Fatalf("expected PUT %s, got %s %s", ledgerPath, got.method, got.path)
	}
	if got.query != "" {
		t.Fatalf("expected no query string, got %q", got.query)
	}
	if !bytes.Equal(got.body, []byte(ledgerBody)) {
		t.Fatalf("body not verbatim:\nwant %s\ngot  %s", ledgerBody, got.body)
	}
}
