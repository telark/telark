package planreports

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/circuitbreaker"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
)

const (
	planID       = "p1"
	okBody       = `{"status":200,"operation":"Success"}`
	failBody     = `{"status":500,"operation":"Failure","message":"boom"}`
	notFoundBody = `{"status":404,"operation":"Failure"}`
	ledgerBody   = `{"planId":"p1","run":"r","checkpoints":[{"at":"t","health":"healthy","violationsSeen":1}],"violations":[]}`
	failedCalls  = 5
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// serve points a fresh ReportClient at a test server and returns the last request body it saw.
func serve(t *testing.T, status int, reply string) (*clients.ReportClient, *[]byte) {
	t.Helper()
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		body = got
		w.WriteHeader(status)
		if _, err := w.Write([]byte(reply)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := clients.NewReportClient()
	client.HTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
	return client, &body
}

func TestClassifyReportResponseMaps4xxToValidationAnd5xxToExporterError(t *testing.T) {
	if err := clients.ClassifyReportResponse(&response.GenericResponse{Status: http.StatusOK}); err != nil {
		t.Fatalf("200: expected nil, got %v", err)
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusRequestEntityTooLarge} {
		err := clients.ClassifyReportResponse(&response.GenericResponse{Status: status, Message: "bad"})
		if !validation.IsValidation(err) || clients.IsExporterFailure(err) {
			t.Fatalf("%d: expected a validation error, got %v", status, err)
		}
	}
	err := clients.ClassifyReportResponse(&response.GenericResponse{Status: http.StatusInternalServerError})
	if !clients.IsExporterFailure(err) || validation.IsValidation(err) {
		t.Fatalf("500: expected an exporter failure, got %v", err)
	}
	err = clients.ClassifyReportResponse(nil)
	if !clients.IsExporterFailure(err) || !strings.Contains(err.Error(), string(constants.ErrReportNilResponse)) {
		t.Fatalf("nil: expected exporter failure with %q, got %v", constants.ErrReportNilResponse, err)
	}
}

// Report calls must never feed the breaker that guards CreateSnapshot.
func TestReportClientNeverTouchesTheExporterBreaker(t *testing.T) {
	manager := circuitbreaker.GetManager()
	manager.Reset(circuitbreaker.DependencyExporter)
	t.Cleanup(func() { manager.Reset(circuitbreaker.DependencyExporter) })

	client, _ := serve(t, http.StatusInternalServerError, failBody)
	for range failedCalls {
		meta, err := client.Create(reportseps.CreatePlanReportRequest{Meta: reportseps.ReportMeta{PlanID: planID}})
		if meta != nil || !clients.IsExporterFailure(err) {
			t.Fatalf("expected exporter failure, got meta=%+v err=%v", meta, err)
		}
	}
	if state := manager.GetState(circuitbreaker.DependencyExporter); state != circuitbreaker.StateClosed {
		t.Fatalf("report calls reached the exporter breaker (state=%v)", state)
	}
}

func TestGetLedgerNotFoundIsNilNil(t *testing.T) {
	client, _ := serve(t, http.StatusNotFound, notFoundBody)

	ledger, err := client.GetLedger(planID)
	if ledger != nil || err != nil {
		t.Fatalf("expected (nil, nil), got (%s, %v)", ledger, err)
	}
}

func TestPutLedgerRoundTripsRealLedgerBytes(t *testing.T) {
	client, body := serve(t, http.StatusOK, okBody)

	if err := client.PutLedger(planID, json.RawMessage(ledgerBody)); err != nil {
		t.Fatalf("PutLedger: %v", err)
	}
	if !bytes.Equal(*body, []byte(ledgerBody)) {
		t.Fatalf("ledger not verbatim:\nwant %s\ngot  %s", ledgerBody, *body)
	}
}
