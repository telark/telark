package namespaces_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dataerrors "github.com/telark/telark/internal/data/errors"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/handlers/namespaces"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// Only the refusals run here: an allowed call lists namespaces from a live cluster.
func TestGetNamespacesRefusesWithoutApplicationsOrInsights(t *testing.T) {
	settingsOwner := xauthz.Identity{Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeSettings: roledata.PermissionLevelOwner},
	}}

	cases := map[string]context.Context{
		"no identity":         context.Background(),
		"settings owner only": xauthz.WithIdentity(context.Background(), settingsOwner),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			namespaces.GetNamespaces(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(ctx))

			testutil.Equal(t, "status", rec.Code, http.StatusForbidden)
			var body struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			testutil.Equal(t, "message", body.Message, string(dataerrors.ErrAuthzForbidden))
		})
	}
}
