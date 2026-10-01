package writes

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
	notifstorage "github.com/telark/telark/services/exporter/internal/redis/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/async"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	callerID      = "u-00010-0000-0010"
	otherOwnerID  = "u-00011-0000-0011"
	storedRoleID  = "r-00002-0000-0002"
	storedGroupID = "ug-00003-0000-0003"
	originalActor = "u-00012-0000-0012"
	statusUserID  = "u-0000f-0000-0006"
	rolesPath     = "/api/v1/accessroles"
	groupsPath    = "/api/v1/groups"
	keyCreatedBy  = "createdBy"
	keyUpdatedBy  = "lastUpdatedBy"
	keyDeletedAt  = "deletedAt"
	keyDesc       = "description"
	testCategory  = "c-00001-0000-0001"
	newDesc       = "changed"
	pathSep       = "/"

	heldRoleA              = "r-00004-0000-0004"
	heldRoleB              = "r-00005-0000-0005"
	grantedRoleID          = "r-00006-0000-0006"
	notificationsListLimit = 10
	roleChangeMessage      = "Granted 1 role. Revoked 2 roles."

	keyIssuedAt = "issuedAt"
	keyIssuedBy = "issuedBy"
)

func adminCaller() xauthz.Identity {
	return xauthz.Identity{UserID: callerID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
}

func rolesOwner(id string) xauthz.Identity {
	return xauthz.Identity{UserID: id, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeRoles: roledata.PermissionLevelOwner},
	}}
}

func jsonBody(t *testing.T, body map[string]any) string {
	t.Helper()
	return string(mustJSON(t, body))
}

func forged(body map[string]any) map[string]any {
	body[keyCreatedBy], body[keyUpdatedBy] = forgedID, forgedID
	return body
}

func storedRole(t *testing.T, protection roledata.Protection) seed {
	t.Helper()
	author := originalActor
	return crSeedOf(t, v1alpha1.AccessRoleMetadata, storedRoleID, roledata.AccessRole{
		Name: appName, Description: newDesc, CategoryRef: testCategory, Type: roledata.RoleTypeCustom,
		Status: roledata.RoleStatusActive, CreatedBy: &author, Protection: &protection,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly}},
	})
}

func storedSpec(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata, name string) map[string]any {
	t.Helper()
	spec, found, err := unstructured.NestedMap(stored(t, client, md, name).Object, keySpec)
	if err != nil || !found {
		t.Fatalf("%s has no spec: %v", name, err)
	}
	return spec
}

// Seen live: a group edit without userRefs removed the group from every
// member, and on the only Admin-granting group answered a false 409.
func TestGroupPatchWithoutMembersKeepsThem(t *testing.T) {
	tests := []struct {
		name, group, member string
		body                map[string]any
		directory           func(t *testing.T) *dynamicfake.FakeDynamicClient
	}{
		{"description of the only admin group", adminGroupID, groupAdminUser, map[string]any{keyDesc: newDesc},
			func(t *testing.T) *dynamicfake.FakeDynamicClient {
				return adminDirectory(t, adminUser(t, groupAdminUser, nil, []string{adminGroupID}, false))
			}},
		{"roles of a mixed group", mixedGroupID, plainUser, map[string]any{keyRoleRefs: []any{}}, mixedDirectory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.directory(t)
			if w := patchAs(t, adminCaller(), groupsPrefix+tt.group, jsonBody(t, tt.body)); w.Code != http.StatusOK {
				t.Fatalf("patch = %d %s", w.Code, w.Body.String())
			}
			if groups := storedList(t, client, v1alpha1.UserMetadata, tt.member, keyGroupRefs); !slices.Contains(groups, tt.group) {
				t.Errorf("member lost the group: %v", groups)
			}
			if members := storedList(t, client, v1alpha1.GroupMetadata, tt.group, keyUserRefs); !slices.Contains(members, tt.member) {
				t.Errorf("group lost the member: %v", members)
			}
		})
	}
}

// Seen live: a profile save, or an admin's role edit, removed the user from every group.
func TestUserPatchWithoutGroupsKeepsMemberships(t *testing.T) {
	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
	}{
		{"own profile", xauthz.Identity{UserID: plainUser}, map[string]any{keyFullname: newName}},
		{"roles by an admin", adminCaller(), map[string]any{keyRoleRefs: []any{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mixedDirectory(t)
			if w := patchAs(t, tt.identity, usersPrefix+plainUser, jsonBody(t, tt.body)); w.Code != http.StatusOK {
				t.Fatalf("patch = %d %s", w.Code, w.Body.String())
			}
			if members := storedList(t, client, v1alpha1.GroupMetadata, mixedGroupID, keyUserRefs); !slices.Contains(members, plainUser) {
				t.Errorf("group lost the member: %v", members)
			}
		})
	}
}

// Seen live: a deleted user stayed in its groups' member lists, and a deleted group in
// its members' groupRefs, until auth's cleanup sweep ran 45-60 s later.
func TestDeleteStripsTheOtherSide(t *testing.T) {
	tests := []struct {
		name, path    string
		identity      xauthz.Identity
		md            base.Metadata
		holder, field string
		want          []string
	}{
		{"user", usersPrefix + plainUser, adminCaller(), v1alpha1.GroupMetadata, mixedGroupID, keyUserRefs, []string{adminA}},
		{"group", groupsPrefix + mixedGroupID, adminCaller(), v1alpha1.UserMetadata, plainUser, keyGroupRefs, nil},
		{"group, by a caller the member is hidden from", groupsPrefix + mixedGroupID, restrictedCaller(),
			v1alpha1.UserMetadata, adminA, keyGroupRefs, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mixedDirectory(t)
			if w := serveAs(t, tt.identity, http.MethodDelete, tt.path, constants.EmptyString); w.Code != http.StatusOK {
				t.Fatalf("delete = %d %s", w.Code, w.Body.String())
			}
			if refs := storedList(t, client, tt.md, tt.holder, tt.field); !slices.Equal(refs, tt.want) {
				t.Errorf("%s %s = %v, want %v", tt.holder, tt.field, refs, tt.want)
			}
		})
	}
}

func statusUser(t *testing.T, phase string, stamp *string) seed {
	t.Helper()
	user := userRecord(statusUserID, nil, nil, false)
	user.Status.Phase, user.Status.LastLoginAt = phase, stamp
	return crSeedOf(t, v1alpha1.UserMetadata, statusUserID, user)
}

// The stamp and the phase have different writers: a login's last-login stamp never
// moves the phase an admin set meanwhile, and a phase change keeps the stamp.
func TestLastLoginStampAndPhaseStayApart(t *testing.T) {
	stamp := stampTime
	tests := []struct {
		name, id string
		seeds    func(t *testing.T) []seed
		caller   xauthz.Identity
		body     map[string]any
		phase    string
	}{
		{"stamp on a suspended account", statusUserID, func(t *testing.T) []seed {
			return []seed{statusUser(t, phaseSuspended, nil)}
		}, xauthz.Identity{Internal: true}, lastLoginStamp(), phaseSuspended},
		{"stamp on the only admin", adminA, func(t *testing.T) []seed {
			return []seed{adminUser(t, adminA, []string{adminRoleID}, nil, false)}
		}, xauthz.Identity{Internal: true}, lastLoginStamp(), phaseActive},
		{"suspension of a stamped account", statusUserID, func(t *testing.T) []seed {
			return []seed{statusUser(t, phaseActive, &stamp)}
		}, adminCaller(), suspend(), phaseSuspended},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := adminDirectory(t, tt.seeds(t)...)
			if w := patchAs(t, tt.caller, usersPrefix+tt.id, jsonBody(t, tt.body)); w.Code != http.StatusOK {
				t.Fatalf("patch = %d %s", w.Code, w.Body.String())
			}
			status, _, err := unstructured.NestedStringMap(storedSpec(t, client, v1alpha1.UserMetadata, tt.id), keyStatus)
			if err != nil || status[keyPhase] != tt.phase || status[keyLastLoginAt] != stampTime {
				t.Fatalf("stored status = %v (%v), want phase %s and lastLoginAt %s", status, err, tt.phase, stampTime)
			}
		})
	}
}

// Auth stamps and clears the invite with a phase-less status, like the last-login stamp:
// the merge patch must store it and clear it (null) while phase and lastLoginAt stay.
func TestInviteStatusIsStoredAndCleared(t *testing.T) {
	stamp := stampTime
	client := adminDirectory(t, statusUser(t, phaseSuspended, &stamp))
	invite := map[string]any{keyIssuedAt: stampTime, keyExpiresAt: stampTime, keyIssuedBy: callerID}
	steps := []struct {
		name   string
		invite map[string]any
	}{
		{"stored", invite},
		{"cleared", nil},
	}
	for _, step := range steps {
		body := map[string]any{constants.FieldStatus: map[string]any{constants.FieldInvite: step.invite}}
		if w := patchAs(t, xauthz.Identity{Internal: true}, usersPrefix+statusUserID, jsonBody(t, body)); w.Code != http.StatusOK {
			t.Fatalf("%s: patch = %d %s", step.name, w.Code, w.Body.String())
		}
		status, _, err := unstructured.NestedMap(storedSpec(t, client, v1alpha1.UserMetadata, statusUserID), keyStatus)
		if err != nil || status[keyPhase] != phaseSuspended || status[keyLastLoginAt] != stampTime {
			t.Fatalf("%s: stored status = %v (%v), want phase %s and lastLoginAt %s kept", step.name, status, err, phaseSuspended, stampTime)
		}
		storedInvite, present := status[constants.FieldInvite].(map[string]any)
		if present != (step.invite != nil) || !maps.Equal(storedInvite, step.invite) {
			t.Fatalf("%s: stored invite = %v, want %v", step.name, status[constants.FieldInvite], step.invite)
		}
	}
}

// Seen live: the bootstrap account's profile save resent its own email and got 403,
// while a typo moved it off the chart's address, where break-glass no longer finds it.
func TestBootstrapEmailStaysTheChartValue(t *testing.T) {
	const reservedEmail, otherEmail = "root@example.com", "test@example.com"
	t.Cleanup(envmanager.InitBootstrapAdmin)
	t.Setenv(constants.BootstrapAdminEnv, reservedEmail)
	envmanager.InitBootstrapAdmin()

	tests := []struct {
		name      string
		bootstrap bool
		stored    string
		body      map[string]any
		want      int
	}{
		{"bootstrap account resends its email", true, reservedEmail,
			map[string]any{keyFullname: newName, constants.FieldEmail: reservedEmail}, http.StatusOK},
		{"bootstrap account takes its email back", true, otherEmail, map[string]any{constants.FieldEmail: reservedEmail}, http.StatusOK},
		{"bootstrap account leaves the chart's email", true, reservedEmail, map[string]any{constants.FieldEmail: otherEmail}, http.StatusForbidden},
		{"another account claims it", false, otherEmail, map[string]any{constants.FieldEmail: reservedEmail}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := userRecord(callerID, nil, nil, tt.bootstrap)
			record.Email = tt.stored
			client := installFake(t, crSeedOf(t, v1alpha1.UserMetadata, callerID, record))
			if w := patchAs(t, xauthz.Identity{UserID: callerID}, usersPrefix+callerID, jsonBody(t, tt.body)); w.Code != tt.want {
				t.Fatalf("patch = %d %s, want %d", w.Code, w.Body.String(), tt.want)
			}
			wantEmail := tt.stored
			if tt.want == http.StatusOK {
				wantEmail = reservedEmail
			}
			if email := storedSpec(t, client, v1alpha1.UserMetadata, callerID)[constants.FieldEmail]; email != wantEmail {
				t.Errorf("stored email = %v, want %s", email, wantEmail)
			}
		})
	}
}

// Seen live: createdBy and lastUpdatedBy were stored as the body gave them,
// and an edit without them was never stamped.
func TestPatchStampsTheCallerAsLastEditor(t *testing.T) {
	author := originalActor
	tests := []struct {
		name string
		md   base.Metadata
		id   string
		path string
		seed func(t *testing.T) seed
	}{
		{"role", v1alpha1.AccessRoleMetadata, storedRoleID, rolesPath, func(t *testing.T) seed {
			return storedRole(t, roledata.Protection{})
		}},
		{"group", v1alpha1.GroupMetadata, storedGroupID, groupsPath, func(t *testing.T) seed {
			return crSeedOf(t, v1alpha1.GroupMetadata, storedGroupID, map[string]any{keyName: appName, keyCreatedBy: author})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, tt.seed(t))
			for _, body := range []map[string]any{{keyDesc: newName}, forged(map[string]any{keyDesc: newDesc})} {
				if w := patchAs(t, adminCaller(), tt.path+pathSep+tt.id, jsonBody(t, body)); w.Code != http.StatusOK {
					t.Fatalf("patch %v = %d %s", body, w.Code, w.Body.String())
				}
				spec := storedSpec(t, client, tt.md, tt.id)
				if spec[keyCreatedBy] != author || spec[keyUpdatedBy] != callerID {
					t.Errorf("after %v: audit = %v / %v, want %s / %s", body, spec[keyCreatedBy], spec[keyUpdatedBy], author, callerID)
				}
			}
		})
	}
}

// protection.softDelete keeps the role on record, marked Deleted, so it grants nothing.
func TestSoftDeleteKeepsTheRoleAsDeleted(t *testing.T) {
	client := installFake(t, storedRole(t, roledata.Protection{SoftDelete: true}))
	if w := serveAs(t, adminCaller(), http.MethodDelete, rolesPath+pathSep+storedRoleID, constants.EmptyString); w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	spec := storedSpec(t, client, v1alpha1.AccessRoleMetadata, storedRoleID)
	if spec[constants.FieldStatus] != string(roledata.RoleStatusDeleted) || spec[keyDeletedAt] == nil || spec[keyUpdatedBy] != callerID {
		t.Errorf("soft-deleted spec = %v", spec)
	}
}

// Only the creator (or an Admin on ALL) may lift a lock, and a lock lifted in
// the same patch no longer blocks the edit it guarded.
func TestRoleLocksAndWhoMayLiftThem(t *testing.T) {
	unlockAndRename := map[string]any{keyName: newName, constants.FieldProtection: map[string]any{constants.FieldLockName: false}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
		want     int
		wantName string
	}{
		{"rename a locked name", adminCaller(), map[string]any{keyName: newName}, http.StatusForbidden, appName},
		{"non-creator lifts the lock", rolesOwner(otherOwnerID), unlockAndRename, http.StatusForbidden, appName},
		{"creator lifts the lock and renames", rolesOwner(originalActor), unlockAndRename, http.StatusOK, newName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := installFake(t, storedRole(t, roledata.Protection{LockName: true}))
			if w := patchAs(t, tt.identity, rolesPath+pathSep+storedRoleID, jsonBody(t, tt.body)); w.Code != tt.want {
				t.Fatalf("patch = %d %s, want %d", w.Code, w.Body.String(), tt.want)
			}
			if got := storedSpec(t, client, v1alpha1.AccessRoleMetadata, storedRoleID)[keyName]; got != tt.wantName {
				t.Errorf("name = %v, want %s", got, tt.wantName)
			}
		})
	}
}

func readOnlyRole(t *testing.T, id string) seed {
	t.Helper()
	return crSeedOf(t, v1alpha1.AccessRoleMetadata, id, roledata.AccessRole{
		Status: roledata.RoleStatusActive, Type: roledata.RoleTypeCustom,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly}},
	})
}

// Seen live: the bell showed "granted 1 role(s); revoked 2 role(s)", against the UI copy rules.
func TestRoleChangeNotificationReadsAsSentences(t *testing.T) {
	exprdb.Set(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(func() { exprdb.Set(nil) })
	async.Init()
	installFake(t, readOnlyRole(t, heldRoleA), readOnlyRole(t, heldRoleB), readOnlyRole(t, grantedRoleID),
		adminUser(t, plainUser, []string{heldRoleA, heldRoleB}, nil, false))

	if w := patchAs(t, adminCaller(), usersPrefix+plainUser, `{"roleRefs":["`+grantedRoleID+`"]}`); w.Code != http.StatusOK {
		t.Fatalf("patch = %d %s", w.Code, w.Body.String())
	}
	async.Drain()
	storage, err := notifstorage.NewStorage()
	if err != nil {
		t.Fatal(err)
	}
	list, err := storage.List(context.Background(), plainUser, notificationsListLimit, constants.EmptyString)
	if err != nil || len(list.Items) != constants.DefaultIncrementValue {
		t.Fatalf("notifications = %v, err %v", list, err)
	}
	if got := list.Items[constants.DefaultInitValue].Message; got != roleChangeMessage {
		t.Errorf("message = %q, want %q", got, roleChangeMessage)
	}
}
