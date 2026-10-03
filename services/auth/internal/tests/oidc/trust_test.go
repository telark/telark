package oidc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	roledata "github.com/telark/telark/internal/data/resources/role"
	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	trustFileName = "googleJwkJson"
	trustFileMode = 0o600
	firstWrite    = 1
	secondWrite   = 2
	oneCall       = 1
	kidOld        = "k-old"
	kidNew        = "k-new"
	contentSynced = "synced"
	contentBroken = "not-json"
	configPath    = "/api/v1/config"
	specKey       = "spec"
	jwkKey        = "googleJwkJson"
	statusKey     = "status"
	dataKey       = "data"
	metadataKey   = "metadata"
	versionKey    = "resourceVersion"
	adminUserID   = "uid"
	callerPath    = "/api/v1/users/" + adminUserID
)

func validJWKS(kid string) string {
	n := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, rsaModulusBytes))
	return fmt.Sprintf(`{"keys":[{"kty":"RSA","use":"sig","kid":%q,"alg":"RS256","n":%q,"e":"AQAB"}]}`, kid, n)
}

// Points the reader at a fresh path and clears any set pinned by an earlier test.
func trustFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), trustFileName)
	t.Setenv(constants.EnvOIDCTrustFile, path)
	oidc.PinTrustJWK(constants.EmptyString)
	t.Cleanup(func() { oidc.PinTrustJWK(constants.EmptyString) })
	return path
}

// Each write gets a distinct mtime, as the kubelet's atomic Secret swap does.
func writeTrust(t *testing.T, path, content string, step int) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), trustFileMode); err != nil {
		t.Fatalf("write: %v", err)
	}
	at := time.Now().Add(time.Duration(step) * time.Minute)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
}

func TestTrustJWKFollowsTheFile(t *testing.T) {
	path := trustFile(t)
	testutil.Equal(t, "missing file", oidc.TrustJWK(), constants.EmptyString)

	writeTrust(t, path, "  "+kidOld+"\n", firstWrite)
	testutil.Equal(t, "read and trimmed", oidc.TrustJWK(), kidOld)

	writeTrust(t, path, kidNew, secondWrite)
	testutil.Equal(t, "re-read on mtime change", oidc.TrustJWK(), kidNew)

	if err := os.Remove(path); err != nil {
		t.Fatalf("remove: %v", err)
	}
	testutil.Equal(t, "file gone", oidc.TrustJWK(), constants.EmptyString)
}

// A set validated in-process is used at once and gives way when the mounted Secret moves.
func TestPinnedJWKWinsUntilTheFileChanges(t *testing.T) {
	path := trustFile(t)
	oidc.PinTrustJWK(kidNew)
	testutil.Equal(t, "no file yet", oidc.TrustJWK(), kidNew)

	writeTrust(t, path, kidOld, firstWrite)
	testutil.Equal(t, "file appears", oidc.TrustJWK(), kidOld)

	oidc.PinTrustJWK(kidNew)
	testutil.Equal(t, "pinned over stale file", oidc.TrustJWK(), kidNew)

	writeTrust(t, path, contentSynced, secondWrite)
	for _, name := range []string{"kubelet sync", "pin cleared"} {
		testutil.Equal(t, name, oidc.TrustJWK(), contentSynced)
	}
}

// Offline, a request that omits the key set is judged against the mounted one.
func TestValidateOfflineUsesTheTrustFile(t *testing.T) {
	path := trustFile(t)
	offline := telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID}

	testutil.Equal(t, "no source", oidc.Validate(offline, false) != nil, true)

	writeTrust(t, path, validJWKS(kidOld), firstWrite)
	testutil.Equal(t, "file source", oidc.Validate(offline, false) != nil, false)
	testutil.Equal(t, "cleared set ignores the file", oidc.Validate(offline, true) != nil, true)

	writeTrust(t, path, contentBroken, secondWrite)
	testutil.Equal(t, "broken file", oidc.Validate(offline, false) != nil, true)

	withSet := offline
	withSet.GoogleJWKJSON = validJWKS(kidNew)
	testutil.Equal(t, "request set wins", oidc.Validate(withSet, true) != nil, false)
}

type exporterRecorder struct {
	mu      sync.Mutex
	patches []map[string]any
	oidc    telarkconfigresource.OIDCConfig
}

// The caller's own record is read too: only the bootstrap account may change the trust.
func (e *exporterRecorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, callerPath) {
		_ = json.NewEncoder(w).Encode(map[string]any{statusKey: http.StatusOK, dataKey: userresource.User{ID: adminUserID, Bootstrap: true}})
		return
	}
	if !strings.HasSuffix(r.URL.Path, configPath) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data := map[string]any{metadataKey: map[string]any{versionKey: kidOld}, telarkconfigresource.FieldOIDC: e.oidc}
	if r.Method == http.MethodPatch {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		e.mu.Lock()
		e.patches = append(e.patches, body)
		e.mu.Unlock()
	}
	_ = json.NewEncoder(w).Encode(map[string]any{statusKey: http.StatusOK, dataKey: data})
}

func (e *exporterRecorder) sentJWK(t *testing.T) any {
	t.Helper()
	return e.sentOIDC(t, oneCall)[jwkKey]
}

func (e *exporterRecorder) sentOIDC(t *testing.T, patches int) map[string]any {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	testutil.Equal(t, "patches", len(e.patches), patches)
	spec, ok := e.patches[patches-oneCall][specKey].(map[string]any)
	if !ok {
		t.Fatalf("patch has no spec: %v", e.patches)
	}
	sent, ok := spec[telarkconfigresource.FieldOIDC].(map[string]any)
	if !ok {
		t.Fatalf("patch has no oidc block: %v", spec)
	}
	return sent
}

// Flags come from the exporter; the key set only from the mounted file.
func TestLoadConfigTakesTheKeySetFromTheFile(t *testing.T) {
	path := trustFile(t)
	testutil.StubBackend(t, &exporterRecorder{oidc: telarkconfigresource.OIDCConfig{
		Enabled: true, GoogleClientID: clientID, GoogleJWKJSON: contentBroken,
	}})

	cfg, err := oidc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	testutil.Equal(t, "client id", cfg.GoogleClientID, clientID)
	testutil.Equal(t, "no file", cfg.GoogleJWKJSON, constants.EmptyString)
	testutil.Equal(t, "not usable offline without a file", oidc.Usable(cfg), false)

	writeTrust(t, path, validJWKS(kidOld), firstWrite)
	cfg, err = oidc.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	testutil.Equal(t, "file set", cfg.GoogleJWKJSON, validJWKS(kidOld))
	testutil.Equal(t, "usable", oidc.Usable(cfg), true)
}

func setConfig(ctx context.Context, cfg telarkconfigresource.OIDCConfig) int {
	body, _ := json.Marshal(cfg)
	return setConfigBody(ctx, string(body))
}

func setConfigBody(ctx context.Context, body string) int {
	r := httptest.NewRequest(http.MethodPatch, configPath, strings.NewReader(body)).WithContext(ctx)
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	rec := httptest.NewRecorder()
	oidchandler.SetConfig(rec, r)
	return rec.Code
}

// SetConfig validates, sends the block to the exporter (which owns the Secret), and
// serves the new set in this replica before the kubelet syncs the mount.
func TestSetConfigPatchesAndPinsTheValidatedSet(t *testing.T) {
	path := trustFile(t)
	writeTrust(t, path, validJWKS(kidOld), firstWrite)
	backend := &exporterRecorder{}
	testutil.StubBackend(t, backend)

	admin := xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin}}
	ctx := xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: adminUserID, Grants: admin})

	status := setConfig(ctx, telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, GoogleJWKJSON: validJWKS(kidNew)})
	testutil.Equal(t, "status", status, http.StatusOK)
	testutil.Equal(t, "jwk sent to exporter", backend.sentJWK(t), any(validJWKS(kidNew)))
	testutil.Equal(t, "in-process set", oidc.TrustJWK(), validJWKS(kidNew))

	status = setConfig(ctx, telarkconfigresource.OIDCConfig{Enabled: true, GoogleClientID: clientID, GoogleJWKJSON: contentBroken})
	testutil.Equal(t, "invalid set refused", status, http.StatusBadRequest)
	testutil.Equal(t, "nothing more stored", backend.sentJWK(t), any(validJWKS(kidNew)))
	testutil.Equal(t, "pin kept", oidc.TrustJWK(), validJWKS(kidNew))
}

// "" (or null) clears the trust set: the exporter is told to drop it, this replica stops
// trusting the still-mounted file at once, and an offline config can no longer lean on it.
// An omitted key keeps the stored set.
func TestSetConfigClearsTheTrustSet(t *testing.T) {
	path := trustFile(t)
	writeTrust(t, path, validJWKS(kidOld), firstWrite)
	backend := &exporterRecorder{}
	testutil.StubBackend(t, backend)
	admin := xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin}}
	ctx := xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: adminUserID, Grants: admin})

	status := setConfigBody(ctx, `{"enabled":true,"googleClientID":"id","egressAllowed":false}`)
	testutil.Equal(t, "omitted key", status, http.StatusOK)
	_, present := backend.sentOIDC(t, firstWrite)[jwkKey]
	testutil.Equal(t, "omitted key not sent", present, false)
	testutil.Equal(t, "file still trusted", oidc.TrustJWK(), validJWKS(kidOld))

	status = setConfigBody(ctx, `{"enabled":false,"googleClientID":"id","googleJwkJson":""}`)
	testutil.Equal(t, "clear", status, http.StatusOK)
	testutil.Equal(t, "clear sent", backend.sentOIDC(t, secondWrite)[jwkKey], any(constants.EmptyString))
	testutil.Equal(t, "stale file no longer trusted", oidc.TrustJWK(), constants.EmptyString)

	status = setConfigBody(ctx, `{"enabled":true,"googleClientID":"id","googleJwkJson":null}`)
	testutil.Equal(t, "offline with the set cleared", status, http.StatusBadRequest)
}
