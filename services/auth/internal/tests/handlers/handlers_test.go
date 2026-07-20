package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/auth/internal/constants"
	authhandler "github.com/telark/auth/internal/handlers/auth"
	authzhandler "github.com/telark/auth/internal/handlers/authorisation"
	cleanuphandler "github.com/telark/auth/internal/handlers/cleanup"
	confighandler "github.com/telark/auth/internal/handlers/config"
	oidchandler "github.com/telark/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/auth/internal/handlers/passkey"
	redishelper "github.com/telark/auth/internal/helpers/redis"
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
	code := m.Run()
	mr.Close()
	os.Exit(code)
}

type handlerFunc func(http.ResponseWriter, *http.Request)

func withSession(r *http.Request) *http.Request {
	r.Header.Set(constants.HeaderSessionToken, "token")
	r.Header.Set(constants.HeaderCredentialID, "cred")
	return r
}

func jsonReq(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	return r
}

func orphanCleanupReq() *http.Request {
	r := jsonReq(`{"forceLastDelete":true,"cleanupOrphaned":true}`)
	r.Header.Set(constants.HeaderUserID, "uid")
	r.Header.Set(constants.HeaderCredentialID, "cred")
	return r
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
		{"login start no backend", authhandler.LoginStart, jsonReq(`{"email":"a@b.com"}`)},
		{"login finish no backend", authhandler.LoginFinish, jsonReq(`{"email":"a@b.com"}`)},
		{"get passkeys", passkeyhandler.GetPasskeys, withSession(jsonReq("{}"))},
		{"create passkey", passkeyhandler.CreatePasskey, jsonReq("{}")},
		{"get single passkey", passkeyhandler.GetSinglePasskey, withSession(jsonReq("{}"))},
		{"update passkey", passkeyhandler.UpdatePasskey, withSession(jsonReq("{}"))},
		{"delete passkey", passkeyhandler.DeletePasskey, jsonReq("{}")},
		{"delete passkey cleanup orphaned", passkeyhandler.DeletePasskey, orphanCleanupReq()},
		{"register start", passkeyhandler.RegisterStart, jsonReq(`{"email":"a@b.com"}`)},
		{"set config bad body", oidchandler.SetConfig, jsonReq("{")},
		{"set config no backend", oidchandler.SetConfig, jsonReq("{}")},
		{"google callback", oidchandler.GoogleCallback, jsonReq("{}")},
		{"permissions no session", authzhandler.GetPermissions, jsonReq("{}")},
		{"delete user no id", cleanuphandler.DeleteUser, jsonReq("{}")},
		{"delete group no id", cleanuphandler.DeleteGroup, jsonReq("{}")},
		{"delete role no id", cleanuphandler.DeleteRole, jsonReq("{}")},
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

// Handlers that must answer even without a session or backend still return 200:
// logout is idempotent, the public config degrades gracefully, and the nonce is
// served from Redis.
func TestHandlersAlwaysOK(t *testing.T) {
	cases := []struct {
		name    string
		handler handlerFunc
		req     *http.Request
	}{
		{"logout", authhandler.Logout, withSession(httptest.NewRequest(http.MethodPost, "/", nil))},
		{"public config", confighandler.GetConfig, httptest.NewRequest(http.MethodGet, "/", nil)},
		{"nonce", oidchandler.GetNonce, httptest.NewRequest(http.MethodGet, "/", nil)},
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
