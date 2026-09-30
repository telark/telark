package planhandlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

func TestGenerateReportRequiresUserHeader(t *testing.T) {
	handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))
	t.Cleanup(func() { handlers.InitService(nil) })

	rec := httptest.NewRecorder()
	handlers.GenerateReport(rec, httptest.NewRequest(http.MethodPost, "/api/v1/protectionplans/plan-a/reports", nil))

	testutil.Equal(t, "status", rec.Code, http.StatusUnauthorized)
}

func TestReportErrorStatusMapsBusyTo429WithRetryAfter(t *testing.T) {
	exporterErr := clients.ClassifyReportResponse(&response.GenericResponse{Status: http.StatusInternalServerError})
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantRetry  int
	}{
		{"busy", reports.ErrRenderBusy, http.StatusTooManyRequests, reports.RenderBusyRetryAfterSec},
		{"wrapped busy", fmt.Errorf("generate: %w", reports.ErrRenderBusy), http.StatusTooManyRequests, reports.RenderBusyRetryAfterSec},
		{"validation", validation.Invalid("not started"), http.StatusBadRequest, constants.DefaultInitValue},
		{"exporter failure", exporterErr, http.StatusServiceUnavailable, constants.DefaultInitValue},
		{"not found", fmt.Errorf("get: %w", clients.ErrPlanNotFound), http.StatusNotFound, constants.DefaultInitValue},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, retry := handlers.ReportErrorStatus(c.err)
			testutil.Equal(t, "status", status, c.wantStatus)
			testutil.Equal(t, "retry-after", retry, c.wantRetry)
		})
	}
}

func TestGenerateReportRouteRequiresWrite(t *testing.T) {
	requirement, found := authz.Requirements()[router.Key(base.Post, planseps.GenerateProtectionPlanReport)]
	if !found {
		t.Fatal("generate-report route has no authz requirement")
	}
	testutil.Equal(t, "min level", requirement.MinLevel, roledata.PermissionLevelContributor)
}
