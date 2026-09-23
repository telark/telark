package snapshotapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/constants"
	snaphandler "github.com/telark/exporter/internal/handlers/snapshot"
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

	get := httptest.NewRecorder()
	snaphandler.GetSnapshot()(get, withID(httptest.NewRequest(http.MethodGet, "/snapshots/app-1"+snapQuery(), nil), testAppID))
	if get.Code != http.StatusOK {
		t.Errorf("get handler code = %d, want 200", get.Code)
	}

	man := httptest.NewRecorder()
	snaphandler.GetSnapshotManifest()(man, withID(httptest.NewRequest(http.MethodGet, "/snapshots/app-1/manifest"+snapQuery(), nil), testAppID))
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
