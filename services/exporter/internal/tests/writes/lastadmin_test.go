package writes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/data/metadata/base"
	"github.com/telark/data/metadata/v1alpha1"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

const (
	adminRoleID    = "r-00000-0000-0001"
	adminGroupID   = "ug-00001-0000-0001"
	adminA         = "u-0000a-0000-0001"
	adminB         = "u-0000b-0000-0002"
	groupAdminUser = "u-0000c-0000-0003"
	bootstrapUser  = "u-0000d-0000-0004"
	plainUser      = "u-0000e-0000-0005"
	phaseSuspended = "suspended"
	keyRoleRefs    = "roleRefs"
	keyGroupRefs   = "groupRefs"
	keyFullname    = "fullname"
)

func crSeedOf(t *testing.T, md base.Metadata, id string, record any) seed {
	t.Helper()
	spec, err := sharedutils.StructToSpecMap(record)
	if err != nil {
		t.Fatal(err)
	}
	return crSeed(md, object(md, id, spec, nil))
}

func userRecord(id string, roles, groups []string, bootstrap bool) *userdata.User {
	user := &userdata.User{ID: id, Status: userdata.UserStatus{Phase: phaseActive}, Bootstrap: bootstrap}
	for i := range roles {
		user.RoleRefs = append(user.RoleRefs, &roles[i])
	}
	for i := range groups {
		user.GroupRefs = append(user.GroupRefs, &groups[i])
	}
	return user
}

func adminUser(t *testing.T, id string, roles, groups []string, bootstrap bool) seed {
	t.Helper()
	return crSeedOf(t, v1alpha1.UserMetadata, id, userRecord(id, roles, groups, bootstrap))
}

func adminDirectory(t *testing.T, users ...seed) {
	t.Helper()
	role := roledata.AccessRole{
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	group := groupdata.Group{RoleRefs: []string{adminRoleID}, UserRefs: []string{groupAdminUser}}
	installFake(t, append(users,
		crSeedOf(t, v1alpha1.AccessRoleMetadata, adminRoleID, role),
		crSeedOf(t, v1alpha1.GroupMetadata, adminGroupID, group),
		adminUser(t, plainUser, nil, nil, false),
	)...)
}

func directAdmin() *userdata.User {
	return userRecord(adminA, []string{adminRoleID}, nil, false)
}

func groupMember() *userdata.User {
	return userRecord(groupAdminUser, nil, []string{adminGroupID}, false)
}

func adminGroup() *groupdata.Group {
	return &groupdata.Group{ID: adminGroupID, RoleRefs: []string{adminRoleID}, UserRefs: []string{groupAdminUser}}
}

func suspend() map[string]any {
	return map[string]any{constants.FieldStatus: map[string]any{keyPhase: phaseSuspended}}
}

type lastAdminCase struct {
	name  string
	users func(t *testing.T) []seed
	guard func(w http.ResponseWriter) bool
	want  bool
}

func lastAdminCases() []lastAdminCase {
	direct := func(t *testing.T) seed { return adminUser(t, adminA, []string{adminRoleID}, nil, false) }
	return []lastAdminCase{
		{"delete one of two admins", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, adminB, []string{adminRoleID}, nil, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, true},
		{"suspend one of two admins", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, adminB, []string{adminRoleID}, nil, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserPatchLastAdmin(w, directAdmin(), suspend()) }, true},
		{"delete an admin while the bootstrap admin remains", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, bootstrapUser, []string{adminRoleID}, nil, true)}
		}, func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, true},
		{"delete the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, directAdmin()) }, false},
		{"suspend the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool { return authz.GuardUserPatchLastAdmin(w, directAdmin(), suspend()) }, false},
		{"demote the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserPatchLastAdmin(w, directAdmin(), map[string]any{keyRoleRefs: []any{}})
			}, false},
		{"profile edit of the last admin", func(t *testing.T) []seed { return []seed{direct(t)} },
			func(w http.ResponseWriter) bool {
				return authz.GuardUserPatchLastAdmin(w, directAdmin(), map[string]any{keyFullname: newName})
			}, true},
		{"leave the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardUserPatchLastAdmin(w, groupMember(), map[string]any{keyGroupRefs: []any{}})
		}, false},
		{"remove the last admin from the group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardGroupPatchLastAdmin(w, adminGroup(), map[string]any{}, []string{groupAdminUser})
		}, false},
		{"strip Admin from the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool {
			return authz.GuardGroupPatchLastAdmin(w, adminGroup(), map[string]any{keyRoleRefs: []any{}}, nil)
		}, false},
		{"delete the only admin group", func(t *testing.T) []seed {
			return []seed{adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardGroupDeleteLastAdmin(w, adminGroupID) }, false},
		{"delete the admin group while a direct admin remains", func(t *testing.T) []seed {
			return []seed{direct(t), adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false)}
		}, func(w http.ResponseWriter) bool { return authz.GuardGroupDeleteLastAdmin(w, adminGroupID) }, true},
		{"no admin to lose", func(*testing.T) []seed { return nil },
			func(w http.ResponseWriter) bool { return authz.GuardUserDeleteLastAdmin(w, userRecord(plainUser, nil, nil, false)) }, true},
	}
}

// Any change that would leave no active user holding Admin on ALL, directly or
// through a group, is refused with 409; the bootstrap account counts as one.
func TestLastAdminGuard(t *testing.T) {
	for _, c := range lastAdminCases() {
		t.Run(c.name, func(t *testing.T) {
			adminDirectory(t, c.users(t)...)
			w := httptest.NewRecorder()
			if got := c.guard(w); got != c.want {
				t.Fatalf("guard = %v, want %v (%d %s)", got, c.want, w.Code, w.Body.String())
			}
			if !c.want && w.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
			}
		})
	}
}
