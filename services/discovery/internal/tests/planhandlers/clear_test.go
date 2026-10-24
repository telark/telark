package planhandlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	keepReportsTypo = "yes"
	notFoundBody    = `{"status":404}`
)

// The refusal comes before the plan lock and the exporter, so a typo never deletes the reports.
func TestClearRefusesAnInvalidKeepReports(t *testing.T) {
	handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))
	t.Cleanup(func() { handlers.InitService(nil) })
	req := planRequest(http.MethodDelete, "")
	req.URL.RawQuery = url.Values{planseps.QueryKeepReports: {keepReportsTypo}}.Encode()

	rec := httptest.NewRecorder()
	handlers.Clear(rec, req)
	testutil.Equal(t, "status", rec.Code, http.StatusBadRequest)
}

// A plan the exporter no longer has answers 404 before anything is cleaned up.
func TestClearAnswersNotFoundForAMissingPlan(t *testing.T) {
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(notFoundBody))}, nil
	})
	handlers.InitService(protection.NewService(nil, nil, clients.NewProtectionPlanClient(), nil, nil, nil, nil, nil))
	t.Cleanup(func() {
		http.DefaultTransport = prev
		handlers.InitService(nil)
	})

	rec := httptest.NewRecorder()
	handlers.Clear(rec, planRequest(http.MethodDelete, ""))
	testutil.Equal(t, "status", rec.Code, http.StatusNotFound)
}
