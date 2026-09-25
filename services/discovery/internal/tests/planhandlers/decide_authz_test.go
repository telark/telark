package planhandlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	dataerrors "github.com/telark/data/errors"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	handlers "github.com/telark/discovery/internal/handlers/plans/protection"
	"github.com/telark/discovery/internal/tests/testutil"
	xauthz "github.com/telark/x-ware/authz"
)

func decide(t *testing.T, decision string, identity *xauthz.Identity) *httptest.ResponseRecorder {
	t.Helper()
	handlers.InitService(protection.NewService(nil, nil, nil, nil, nil, nil, nil, nil))
	t.Cleanup(func() { handlers.InitService(nil) })

	body := fmt.Sprintf(`{"decision":%q,"comment":"c","requestedAt":"2026-09-23T00:00:00Z"}`, decision)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans/protection/plan-a/decide", strings.NewReader(body))
	req.Header.Set(constants.HeaderUserID, "u-approver")
	req = mux.SetURLVars(req, map[string]string{constants.IDPathParam: "plan-a"})
	if identity != nil {
		req = req.WithContext(xauthz.WithIdentity(req.Context(), *identity))
	}

	rec := httptest.NewRecorder()
	handlers.Decide(rec, req)
	return rec
}

func ownerDenied(action string) *xauthz.Identity {
	plans := roledata.ScopeProtectionPlans
	return &xauthz.Identity{
		UserID: "u-approver",
		Grants: xauthz.Grants{
			Levels: map[string]roledata.PermissionLevel{plans: roledata.PermissionLevelOwner},
			Denied: map[string][]string{plans: {xauthz.RuleKey(plans, action)}},
		},
	}
}

// The service has no exporter, so reaching svc.Decide would not answer 403:
// the refusal must come before the lock and the exporter call.
func TestDecideRefusesDeniedDecision(t *testing.T) {
	cases := []struct {
		name     string
		decision string
		identity *xauthz.Identity
	}{
		{"approve withheld", protection.DecisionApproved, ownerDenied(roledata.ActionApproveProtectionPlan)},
		{"reject withheld", protection.DecisionRejected, ownerDenied(roledata.ActionRejectProtectionPlan)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := decide(t, c.decision, c.identity)
			testutil.Equal(t, "status", rec.Code, http.StatusForbidden)
			testutil.Equal(t, "forbidden message", strings.Contains(rec.Body.String(), string(dataerrors.ErrAuthzForbidden)), true)
		})
	}
}

func TestDecideRefusesMissingIdentity(t *testing.T) {
	rec := decide(t, protection.DecisionApproved, nil)
	testutil.Equal(t, "status", rec.Code, http.StatusForbidden)
}
