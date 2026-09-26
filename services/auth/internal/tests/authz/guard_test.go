package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/tests/testutil"
	roledata "github.com/telark/data/resources/role"
	userresource "github.com/telark/data/resources/user"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	bootstrapID = "u-boot"
	adminID     = "u-admin"
	ownerID     = "u-owner"
	plainID     = "u-plain"
	usersPath   = "/users/findbyid/"
	rolesPath   = "/roles/"
)

var adminRoleID = constants.BuiltInRoleAdmin

// A directory of users and the built-in Admin role, served the way the exporter
// serves single-resource reads; unknown ids answer 404.
func stubDirectory(t *testing.T) {
	t.Helper()
	suspended := userresource.UserStatus{Phase: string(userresource.AccountPhaseSuspended)}
	users := map[string]*userresource.UserAsResource{
		bootstrapID: {ID: bootstrapID, Bootstrap: true, AssignedRolesIDs: []*string{&adminRoleID}},
		adminID:     {ID: adminID, AssignedRolesIDs: []*string{&adminRoleID}, Status: suspended},
		ownerID:     {ID: ownerID},
		plainID:     {ID: plainID},
	}
	admin := roledata.RoleAsResource{
		ID: adminRoleID, Status: roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload any
		switch {
		case strings.Contains(r.URL.Path, usersPath):
			id := strings.TrimSuffix(r.URL.Path[strings.Index(r.URL.Path, usersPath)+len(usersPath):], "/get")
			user, ok := users[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			payload = user
		case strings.Contains(r.URL.Path, rolesPath):
			payload = admin
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
	}))
}

type caller struct {
	userID    string
	level     roledata.PermissionLevel
	internal  bool
	anonymous bool
}

func (c caller) ctx() context.Context {
	if c.anonymous {
		return context.Background()
	}
	grants := xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: c.level}}
	return xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: c.userID, Internal: c.internal, Grants: grants})
}

// Bootstrap users are never deleted through the API, administrators only by a
// bootstrap user, and a caller below Admin is not told an administrator exists.
// A suspended administrator is still one. Internal callers are not gated.
func TestGuardUserDelete(t *testing.T) {
	stubDirectory(t)
	owner := caller{userID: ownerID, level: roledata.PermissionLevelOwner}
	cases := []struct {
		name       string
		caller     caller
		target     string
		wantStatus int
	}{
		{"self", owner, ownerID, http.StatusForbidden},
		{"owner deletes plain user", owner, plainID, http.StatusOK},
		{"owner targets admin", owner, adminID, http.StatusNotFound},
		{"owner targets bootstrap", owner, bootstrapID, http.StatusNotFound},
		{"admin targets bootstrap", caller{userID: adminID, level: roledata.PermissionLevelAdmin}, bootstrapID, http.StatusForbidden},
		{"admin targets admin", caller{userID: plainID, level: roledata.PermissionLevelAdmin}, adminID, http.StatusForbidden},
		{"bootstrap deletes admin", caller{userID: bootstrapID, level: roledata.PermissionLevelAdmin}, adminID, http.StatusOK},
		{"missing target", owner, "u-none", http.StatusNotFound},
		{"internal", caller{internal: true}, bootstrapID, http.StatusOK},
		{"no identity", caller{anonymous: true}, plainID, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := authz.GuardUserDelete(c.caller.ctx(), c.target)
			testutil.Equal(t, "status", status, c.wantStatus)
			testutil.Equal(t, "refused", err != nil, c.wantStatus != http.StatusOK)
		})
	}
}
