package writes

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	namesPath     = "/api/v1/users/names?ids="
	adminUsername = "admin-a"
	plainUsername = "plain-user"
	generatedID   = "u-%05d-0000-0000"
)

func namedUser(t *testing.T, id, username string, roles []string) seed {
	t.Helper()
	record := userRecord(id, roles, nil, false)
	record.Username = username
	return crSeedOf(t, v1alpha1.UserMetadata, id, record)
}

// The caller holds no grants at all: the route needs only a session.
func namesAs(t *testing.T, ids ...string) *httptest.ResponseRecorder {
	t.Helper()
	query := url.QueryEscape(strings.Join(ids, constants.UserIDsSeparator))
	return serveAs(t, xauthz.Identity{UserID: restrictedID}, http.MethodGet, namesPath+query, constants.EmptyString)
}

func distinctIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf(generatedID, i)
	}
	return ids
}

// Audit actors are named for every viewer: an administrator hidden from a
// restricted caller is still named, and an unknown id is left out.
func TestUserNamesResolveForCallerWithoutGrants(t *testing.T) {
	role := roledata.AccessRole{
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	installFake(t,
		crSeedOf(t, v1alpha1.AccessRoleMetadata, adminRoleID, role),
		namedUser(t, adminA, adminUsername, []string{adminRoleID}),
		namedUser(t, plainUser, plainUsername, nil),
	)

	w := namesAs(t, adminA, " "+plainUser, plainUser, missingUserID, constants.EmptyString)

	if w.Code != http.StatusOK {
		t.Fatalf("names = %d %s", w.Code, w.Body.String())
	}
	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{adminA: adminUsername, plainUser: plainUsername}; !maps.Equal(body.Data, want) {
		t.Errorf("names = %v, want %v", body.Data, want)
	}
}

func TestUserNamesBoundsTheRequest(t *testing.T) {
	installFake(t)
	overCap := constants.UserNamesMaxIDs + constants.DefaultIncrementValue
	for _, tc := range []struct {
		name string
		ids  []string
		want int
	}{
		{"no ids", nil, http.StatusBadRequest},
		{"only blanks", []string{" ", constants.EmptyString}, http.StatusBadRequest},
		{"at the cap", distinctIDs(constants.UserNamesMaxIDs), http.StatusOK},
		{"over the cap", distinctIDs(overCap), http.StatusBadRequest},
		{"repeats count once", slices.Repeat([]string{plainUser}, overCap), http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := namesAs(t, tc.ids...); got.Code != tc.want {
				t.Errorf("%s = %d %s, want %d", tc.name, got.Code, got.Body.String(), tc.want)
			}
		})
	}
}
