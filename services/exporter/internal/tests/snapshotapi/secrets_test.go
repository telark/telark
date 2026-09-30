package snapshotapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	expsnap "github.com/telark/telark/services/exporter/internal/exporters/snapshot"
	snaphandler "github.com/telark/telark/services/exporter/internal/handlers/snapshot"
)

const (
	fieldData    = "data"
	keyToken     = "token"
	dummyEncoded = "ZHVtbXk="
)

func secretBody() map[string]any {
	body := manifestBody()
	body[constants.FieldManifest] = map[string]any{
		constants.FieldResources: []any{
			map[string]any{constants.FieldManifest: map[string]any{
				constants.FieldKind: constants.KindSecret,
				fieldData:           map[string]any{keyToken: dummyEncoded},
			}},
		},
	}
	return body
}

func createSecretSnapshot(t *testing.T) {
	t.Helper()
	setRoot(t)
	rec := httptest.NewRecorder()
	expsnap.CreateSnapshot(rec, secretBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("CreateSnapshot code = %d, want 200", rec.Code)
	}
}

func assertBody(t *testing.T, rec *httptest.ResponseRecorder, redacted bool, what string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: code = %d, want 200: %s", what, rec.Code, rec.Body.String())
	}
	hasValue := strings.Contains(rec.Body.String(), dummyEncoded)
	hasMarker := strings.Contains(rec.Body.String(), constants.SecretValueRedacted)
	if redacted && (hasValue || !hasMarker) {
		t.Errorf("%s: secret value left in the response", what)
	}
	if !redacted && (!hasValue || hasMarker) {
		t.Errorf("%s: secret value was masked for an internal caller", what)
	}
}

func TestReadSnapshotRedactsSecretsForSessions(t *testing.T) {
	createSecretSnapshot(t)

	for _, accept := range []string{constants.EmptyString, "application/yaml"} {
		session := httptest.NewRecorder()
		expsnap.ReadSnapshotManifestWithAccept(session, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, accept, true)
		assertBody(t, session, true, "manifest "+accept)

		internal := httptest.NewRecorder()
		expsnap.ReadSnapshotManifestWithAccept(internal, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, accept, false)
		assertBody(t, internal, false, "manifest "+accept)
	}

	session := httptest.NewRecorder()
	expsnap.ReadSnapshot(session, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, true)
	assertBody(t, session, true, "get")

	// The stored file keeps the real value, so a rollback still restores it.
	internal := httptest.NewRecorder()
	expsnap.ReadSnapshot(internal, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, false)
	assertBody(t, internal, false, "get")
}

func getAs(identity xauthz.Identity) *http.Request {
	r := withID(httptest.NewRequest(http.MethodGet, "/snapshots/"+testAppID+snapQuery(), nil), testAppID)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

func TestGetSnapshotHandlerHonoursManifestDenyAndRedacts(t *testing.T) {
	createSecretSnapshot(t)
	reader := xauthz.Identity{UserID: "u1", Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeApplications: roledata.PermissionLevelReadOnly},
	}}
	denied := reader
	denied.Grants.Denied = map[string][]string{roledata.ScopeApplications: {
		xauthz.RuleKey(roledata.ScopeApplications, roledata.ActionViewApplicationSnapshotManifest),
	}}

	rec := httptest.NewRecorder()
	snaphandler.GetSnapshot()(rec, getAs(denied))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("manifest-denied get code = %d, want 403", rec.Code)
	}

	rec = httptest.NewRecorder()
	snaphandler.GetSnapshot()(rec, getAs(reader))
	assertBody(t, rec, true, "session get")
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil || payload.Data[constants.FieldManifest] == nil {
		t.Fatalf("session get lost the manifest: %v %s", err, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	snaphandler.GetSnapshot()(rec, getAs(xauthz.Identity{Internal: true}))
	assertBody(t, rec, false, "internal get")

	rec = httptest.NewRecorder()
	snaphandler.GetSnapshotManifest()(rec, getAs(reader))
	assertBody(t, rec, true, "session manifest")
}
