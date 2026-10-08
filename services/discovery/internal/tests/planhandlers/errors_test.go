package planhandlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/circuitbreaker"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	handlers "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	wrapDecide     = "decide: %w"
	wrapPrepare    = "prepare: %w"
	errorLogMarker = "[ERROR]"
)

// Every domain failure carries its own type, so the status comes from the error itself and
// never from matching its message.
func TestStatusForErr(t *testing.T) {
	k8sErr := k8serrors.NewConflict(schema.GroupResource{Group: "kyverno.io", Resource: "policies"}, "pol-a", errors.New("conflict"))
	taken := []plans.ProtectionPlan{{ID: "pp-1", Name: "guard"}}
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, http.StatusOK},
		{"not found", fmt.Errorf("get plan: %w", clients.ErrPlanNotFound), http.StatusNotFound},
		{"validation", validation.Invalid("bad request"), http.StatusBadRequest},
		{"wrapped validation", fmt.Errorf("prepare: %w", validation.ErrPoliciesRequired), http.StatusBadRequest},
		{"breaker open", fmt.Errorf("list plans: %w", circuitbreaker.ErrOpen), http.StatusServiceUnavailable},
		{"cluster error", fmt.Errorf("apply: %w", k8sErr), http.StatusServiceUnavailable},
		{"self decision", fmt.Errorf(wrapDecide, protection.ErrDecisionSelf), http.StatusForbidden},
		{"not pending", fmt.Errorf(wrapDecide, protection.ErrDecisionNotPending), http.StatusConflict},
		{"stale", fmt.Errorf(wrapDecide, protection.ErrDecisionStale), http.StatusConflict},
		{"name taken", validation.UniqueName(taken, "guard", constants.EmptyString), http.StatusConflict},
		{"name in flight", fmt.Errorf("prepare: %w", protection.ErrNameInFlight), http.StatusConflict},
		{"excluded list unavailable", fmt.Errorf(wrapPrepare, &validation.UnavailableError{Msg: "down"}), http.StatusServiceUnavailable},
		{"enforce needs owner", fmt.Errorf(wrapPrepare, validation.ErrEnforceNeedsOwner), http.StatusForbidden},
		{"unknown environment", validation.EnvironmentRef(strptr("cat-9"), func() ([]string, error) { return nil, nil }), http.StatusBadRequest},
		{"unknown", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "status", handlers.StatusForErr(c.err), c.want)
		})
	}
}

// Seen live: every bad body, missing caller and unknown plan printed an [ERROR] line. A 5xx still logs.
func TestPlanRefusalsAreNotLoggedAsErrors(t *testing.T) {
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Header: http.Header{}, Body: http.NoBody}, nil
	})
	handlers.InitService(protection.NewService(nil, nil, clients.NewProtectionPlanClient(), nil, nil, nil, nil, nil))
	t.Cleanup(func() {
		http.DefaultTransport = prev
		handlers.InitService(nil)
	})
	noCaller := planRequest(http.MethodPost, `{"name":"p"}`)
	noCaller.Header.Del(constants.HeaderUserID)
	cases := []struct {
		name    string
		handler http.HandlerFunc
		req     *http.Request
		want    int
	}{
		{"malformed body", handlers.Prepare, planRequest(http.MethodPost, `{"name":"p","approvalGate":"off"}`), http.StatusBadRequest},
		{"no caller", handlers.Prepare, noCaller, http.StatusUnauthorized},
		{"unknown plan", handlers.Status, planRequest(http.MethodGet, constants.EmptyString), http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			out := testutil.CaptureStdout(t, func() { c.handler(rec, c.req) })
			if rec.Code != c.want || strings.Contains(out, errorLogMarker) {
				t.Fatalf("answered %d (want %d), logged %q", rec.Code, c.want, out)
			}
		})
	}

	handlers.InitService(nil)
	rec := httptest.NewRecorder()
	out := testutil.CaptureStdout(t, func() { handlers.Prepare(rec, planRequest(http.MethodPost, `{"name":"p"}`)) })
	testutil.Equal(t, "service not ready", rec.Code, http.StatusServiceUnavailable)
	testutil.Equal(t, "a 5xx still logs", strings.Contains(out, errorLogMarker), true)
}
