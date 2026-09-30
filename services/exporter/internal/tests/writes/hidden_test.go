package writes

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	mixedGroupID  = "ug-00002-0000-0002"
	restrictedID  = "u-0000f-0000-0006"
	missingUserID = "u-fffff-0000-0000"
	groupsPrefix  = "/api/v1/groups/"
	usersPrefix   = "/api/v1/users/"
	keyUserRefs   = "userRefs"
	malformedBody = `{"Fullname":"x"}`
)

// adminA is a hidden member of the mixed group, adminB a hidden non-member.
func mixedDirectory(t *testing.T) *dynamicfake.FakeDynamicClient {
	t.Helper()
	role := roledata.AccessRole{
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	return installFake(t,
		crSeedOf(t, v1alpha1.AccessRoleMetadata, adminRoleID, role),
		crSeedOf(t, v1alpha1.GroupMetadata, mixedGroupID, groupdata.Group{UserRefs: []string{plainUser, adminA}}),
		adminUser(t, adminA, []string{adminRoleID}, []string{mixedGroupID}, false),
		adminUser(t, adminB, []string{adminRoleID}, nil, false),
		adminUser(t, plainUser, nil, []string{mixedGroupID}, false),
	)
}

func restrictedCaller() xauthz.Identity {
	return xauthz.Identity{UserID: restrictedID, Grants: xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
		roledata.ScopeGroups: roledata.PermissionLevelOwner,
		roledata.ScopeUsers:  roledata.PermissionLevelOwner,
	}}}
}

func patchAs(t *testing.T, identity xauthz.Identity, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return serveAs(t, identity, http.MethodPatch, path, body)
}

func serveAs(t *testing.T, identity xauthz.Identity, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r = r.WithContext(xauthz.WithIdentity(r.Context(), identity))
	w := httptest.NewRecorder()
	router.NewRouter(routes.InitRoutes(optimizer)).ServeHTTP(w, r)
	return w
}

func membersBody(ids ...string) string {
	return `{"` + keyUserRefs + `":["` + strings.Join(ids, `","`) + `"]}`
}

func storedList(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata, name, field string) []string {
	t.Helper()
	list, _, _ := unstructured.NestedStringSlice(stored(t, client, md, name).Object, keySpec, field)
	return list
}

// Seen live: a restricted groups Owner saving the member list they were shown
// dropped the administrators hidden from it.
func TestRestrictedGroupSaveKeepsHiddenMembers(t *testing.T) {
	client := mixedDirectory(t)

	w := patchAs(t, restrictedCaller(), groupsPrefix+mixedGroupID, membersBody(plainUser))

	if w.Code != http.StatusOK {
		t.Fatalf("save = %d %s", w.Code, w.Body.String())
	}
	if members := storedList(t, client, v1alpha1.GroupMetadata, mixedGroupID, keyUserRefs); !slices.Contains(members, adminA) {
		t.Errorf("hidden member dropped: %v", members)
	}
	if groups := storedList(t, client, v1alpha1.UserMetadata, adminA, keyGroupRefs); !slices.Contains(groups, mixedGroupID) {
		t.Errorf("hidden member lost the group: %v", groups)
	}
}

// Seen live: adding a hidden administrator by id answered 200 where a missing
// id answers 400, and wrote to the administrator's groupRefs.
func TestRestrictedGroupEditAnswersHiddenLikeMissing(t *testing.T) {
	client := mixedDirectory(t)
	patch := func(id string) *httptest.ResponseRecorder {
		return patchAs(t, restrictedCaller(), groupsPrefix+mixedGroupID, membersBody(plainUser, id))
	}

	missing := patch(missingUserID)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing id = %d %s, want 400", missing.Code, missing.Body.String())
	}
	// adminB is not a member; adminA is one, hidden from the caller.
	for _, id := range []string{adminB, adminA} {
		if got := patch(id); got.Code != missing.Code || strings.ReplaceAll(got.Body.String(), id, missingUserID) != missing.Body.String() {
			t.Errorf("hidden %s = %d %s, missing = %s", id, got.Code, got.Body.String(), missing.Body.String())
		}
	}
	if groups := storedList(t, client, v1alpha1.UserMetadata, adminB, keyGroupRefs); len(groups) > constants.DefaultInitValue {
		t.Errorf("hidden administrator was written: %v", groups)
	}
}

// Seen live: an invalid body on PATCH users/{hidden administrator} answered 400
// where a missing id answers 404.
func TestUserPatchAnswersHiddenAdminLikeMissing(t *testing.T) {
	mixedDirectory(t)

	missing := patchAs(t, restrictedCaller(), usersPrefix+missingUserID, malformedBody)
	hidden := patchAs(t, restrictedCaller(), usersPrefix+adminA, malformedBody)
	if missing.Code != http.StatusNotFound || hidden.Code != missing.Code || hidden.Body.String() != missing.Body.String() {
		t.Fatalf("hidden = %d %s, missing = %d %s", hidden.Code, hidden.Body.String(), missing.Code, missing.Body.String())
	}

	admin := xauthz.Identity{UserID: restrictedID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
	if seen := patchAs(t, admin, usersPrefix+adminA, malformedBody); seen.Code != http.StatusBadRequest {
		t.Fatalf("admin with a malformed body = %d, want 400", seen.Code)
	}
}
