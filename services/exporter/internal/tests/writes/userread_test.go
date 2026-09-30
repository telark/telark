package writes

import (
	"net/http"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

// Seen live: a user without the users scope got 403 reading their own profile,
// so the UI could never refresh it.
func TestUserReadIsSelfOrUsersReadOnly(t *testing.T) {
	installFake(t, namedUser(t, plainUser, plainUsername, nil))
	stranger := xauthz.Identity{UserID: restrictedID}
	usersReader := xauthz.Identity{UserID: restrictedID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeUsers: roledata.PermissionLevelReadOnly},
	}}
	get := func(caller xauthz.Identity, id string) (int, string) {
		w := serveAs(t, caller, http.MethodGet, usersPrefix+id, constants.EmptyString)
		return w.Code, w.Body.String()
	}

	if code, body := get(xauthz.Identity{UserID: plainUser}, plainUser); code != http.StatusOK || !strings.Contains(body, plainUsername) {
		t.Errorf("own record = %d %s, want 200 with the profile", code, body)
	}
	if code, body := get(usersReader, plainUser); code != http.StatusOK || !strings.Contains(body, plainUsername) {
		t.Errorf("users reader = %d %s, want 200 with the profile", code, body)
	}

	otherCode, otherBody := get(stranger, plainUser)
	missingCode, missingBody := get(stranger, missingUserID)
	if otherCode != http.StatusForbidden || missingCode != otherCode || missingBody != otherBody {
		t.Errorf("another record = %d %s, missing = %d %s, want the same 403", otherCode, otherBody, missingCode, missingBody)
	}
}
