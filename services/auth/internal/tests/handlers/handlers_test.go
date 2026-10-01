package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/telark/services/auth/internal/constants"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	authzhandler "github.com/telark/telark/services/auth/internal/handlers/authorisation"
	cleanuphandler "github.com/telark/telark/services/auth/internal/handlers/cleanup"
	confighandler "github.com/telark/telark/services/auth/internal/handlers/config"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	testPath  = "/"
	emptyJSON = "{}"
	emailBody = `{"email":"a@b.com"}`
)

// Redis backs the nonce handler; the resource backend is deliberately absent so
// handlers that reach it exercise their failure paths.
func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	redishelper.NewRedisClientWithRetry(context.Background())
	m.Run()
	mr.Close()
}

type handlerFunc func(http.ResponseWriter, *http.Request)

func withSession(r *http.Request) *http.Request {
	r.Header.Set(constants.HeaderSessionToken, "token")
	return testutil.WithCredentialID(r, "cred")
}

func jsonReq(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader(body))
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	return r
}

func orphanCleanupReq() *http.Request {
	r := jsonReq(`{"forceLastDelete":true,"cleanupOrphaned":true}`)
	r.Header.Set(constants.HeaderUserID, "uid")
	return testutil.WithCredentialID(r, "cred")
}

// Without a reachable backend (and no valid session), every guarded handler
// answers with a client or server error rather than a success.
func TestHandlersFailClosed(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		req     *http.Request
	}{
		{"login start bad body", authhandler.LoginStart, jsonReq("{")},
		{"login start no backend", authhandler.LoginStart, jsonReq(emailBody)},
		{"login finish no backend", authhandler.LoginFinish, jsonReq(emailBody)},
		{"get passkeys", passkeyhandler.GetPasskeys, withSession(jsonReq(emptyJSON))},
		{"create passkey", passkeyhandler.CreatePasskey, jsonReq(emptyJSON)},
		{"get single passkey", passkeyhandler.GetSinglePasskey, withSession(jsonReq(emptyJSON))},
		{"update passkey", passkeyhandler.UpdatePasskey, withSession(jsonReq(emptyJSON))},
		{"delete passkey", passkeyhandler.DeletePasskey, jsonReq(emptyJSON)},
		{"delete passkey cleanup orphaned", passkeyhandler.DeletePasskey, orphanCleanupReq()},
		{"register start", passkeyhandler.RegisterStart, jsonReq(emailBody)},
		{"set config bad body", oidchandler.SetConfig, jsonReq("{")},
		{"set config no backend", oidchandler.SetConfig, jsonReq(emptyJSON)},
		{"google callback", oidchandler.GoogleCallback, jsonReq(emptyJSON)},
		{"permissions no session", authzhandler.GetPermissions, jsonReq(emptyJSON)},
		{"delete user no id", cleanuphandler.DeleteUser, jsonReq(emptyJSON)},
		{"delete group no id", cleanuphandler.DeleteGroup, jsonReq(emptyJSON)},
		{"delete role no id", cleanuphandler.DeleteAccessRole, jsonReq(emptyJSON)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.handler(rec, c.req)
			if rec.Code < http.StatusBadRequest {
				t.Fatalf("%s = %d, want an error status (>=400)", c.name, rec.Code)
			}
		})
	}
}

// A body over the cap is refused with 413 wherever it is read, before any lookup.
func TestOversizedBodyIsRefused(t *testing.T) {
	body := `{"email":"` + strings.Repeat("a", constants.MaxRequestBodyBytes) + `"}`
	cases := map[string]handlerFunc{
		"login start":    authhandler.LoginStart,
		"login finish":   authhandler.LoginFinish,
		"register start": passkeyhandler.RegisterStart,
		"create passkey": passkeyhandler.CreatePasskey,
		"delete passkey": passkeyhandler.DeletePasskey,
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, jsonReq(body))
			testutil.Equal(t, "status", rec.Code, http.StatusRequestEntityTooLarge)
		})
	}
}

// Handlers that must answer even without a session or backend still return 200:
// logout is idempotent, the public config degrades gracefully, and the nonce is
// served from Redis.
func TestHandlersAlwaysOK(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		req     *http.Request
	}{
		{"logout", authhandler.Logout, withSession(httptest.NewRequest(http.MethodPost, testPath, nil))},
		{"public config", confighandler.GetConfig, httptest.NewRequest(http.MethodGet, testPath, nil)},
		{"nonce", oidchandler.GetNonce, httptest.NewRequest(http.MethodGet, testPath, nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c.handler(rec, c.req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d, want 200", c.name, rec.Code)
			}
		})
	}
}
