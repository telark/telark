package snapshotapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	snaphandler "github.com/telark/exporter/internal/handlers/snapshot"
	xauthz "github.com/telark/x-ware/authz"
)

func snapQuery() string {
	q := url.Values{}
	q.Set(constants.ScopeParam, constants.SnapshotsAppsSubdir)
	q.Set(constants.NamespaceParam, testNamespace)
	q.Set(constants.GenerationParam, testGeneration)
	return "?" + q.Encode()
}

func withID(r *http.Request, id string) *http.Request {
	return mux.SetURLVars(r, map[string]string{constants.IDParam: id})
}

func TestCreateSnapshotHandler(t *testing.T) {
	setRoot(t)
	body := `{"id":"app-1","scope":"apps","namespace":"ns","generation":1,"manifest":{"resources":[]}}`
	rec := httptest.NewRecorder()
	snaphandler.CreateSnapshot()(rec, httptest.NewRequest(http.MethodPost, "/snapshots", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Errorf("create handler code = %d, want 200", rec.Code)
	}

	bad := httptest.NewRecorder()
	snaphandler.CreateSnapshot()(bad, httptest.NewRequest(http.MethodPost, "/snapshots", strings.NewReader("{bad")))
	if bad.Code != http.StatusUnprocessableEntity {
		t.Errorf("bad body code = %d, want 422", bad.Code)
	}
}

func TestSnapshotReadHandlers(t *testing.T) {
	setRoot(t)
	createOK(t)

	// The snapshot body and the manifest are each held to the other's rule too, so both need a caller.
	get := httptest.NewRecorder()
	snaphandler.GetSnapshot()(get, getAs(xauthz.Identity{Internal: true}))
	if get.Code != http.StatusOK {
		t.Errorf("get handler code = %d, want 200", get.Code)
	}

	man := httptest.NewRecorder()
	snaphandler.GetSnapshotManifest()(man, getAs(xauthz.Identity{Internal: true}))
	if man.Code != http.StatusOK {
		t.Errorf("manifest handler code = %d, want 200", man.Code)
	}

	infos := httptest.NewRecorder()
	snaphandler.GetSnapshotInfos()(infos, httptest.NewRequest(http.MethodGet, "/snapshots/infos", nil))
	if infos.Code != http.StatusOK {
		t.Errorf("infos handler code = %d, want 200", infos.Code)
	}
}

func TestDeleteSnapshotHandler(t *testing.T) {
	setRoot(t)
	createOK(t)
	rec := httptest.NewRecorder()
	snaphandler.DeleteSnapshot()(rec, withID(httptest.NewRequest(http.MethodDelete, "/snapshots/app-1"+snapQuery(), nil), testAppID))
	if rec.Code != http.StatusOK {
		t.Errorf("delete handler code = %d, want 200", rec.Code)
	}
}

// Seen live: a deny on viewing snapshots still let the manifest download; the
// manifest rule itself is the route's.
func TestGetSnapshotManifestHonoursSnapshotDeny(t *testing.T) {
	setRoot(t)
	createOK(t)
	reader := xauthz.Identity{UserID: "u1", Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeApplications: roledata.PermissionLevelReadOnly},
	}}
	tests := []struct {
		name   string
		action string
		want   int
	}{
		{"snapshots denied", roledata.ActionViewApplicationsSnapshots, http.StatusForbidden},
		{"nothing denied", constants.EmptyString, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			identity := reader
			if tt.action != constants.EmptyString {
				identity.Grants.Denied = map[string][]string{roledata.ScopeApplications: {
					xauthz.RuleKey(roledata.ScopeApplications, tt.action),
				}}
			}
			rec := httptest.NewRecorder()
			snaphandler.GetSnapshotManifest()(rec, getAs(identity))
			if rec.Code != tt.want {
				t.Fatalf("manifest code = %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}
