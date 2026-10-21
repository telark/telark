package planhandlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const keepReportsTypo = "yes"

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
