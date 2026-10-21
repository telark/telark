package handlers

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/constants"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	sessionHours  = 24
	maxAgeSlackS  = 1
	oneCookie     = 1
	firstCookieAt = 0
)

// The chart's SESSION_COOKIE_HOST_PREFIX value and the one cookie name auth then sets.
var cookieNames = map[string]string{
	"false": dataconstants.CookieSession,
	"true":  dataconstants.CookieSessionHostPrefix + dataconstants.CookieSession,
}

// The session cookie a sign-in answers with, alone: HttpOnly, Secure, Strict, Path=/, host-only, ending with the
// session, and no token in the body.
func expectSessionCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) {
	t.Helper()
	cookies := rec.Result().Cookies()
	if len(cookies) != oneCookie || cookies[firstCookieAt].Name != name ||
		cookies[firstCookieAt].Value == constants.EmptyString {
		t.Fatalf("cookies = %+v, want one %s", cookies, name)
	}
	c := cookies[firstCookieAt]
	want := (sessionHours * time.Hour).Seconds()
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != constants.EmptyString ||
		math.Abs(float64(c.MaxAge)-want) > maxAgeSlackS {
		t.Fatalf("cookie = %+v, want HttpOnly, Secure, Strict, Path=/, no Domain, Max-Age %v", c, want)
	}
	testutil.Equal(t, "token in the body", strings.Contains(rec.Body.String(), "sessionToken"), false)
}

func TestSignInsSetOnlyTheCookie(t *testing.T) {
	initWebAuthn(t)
	t.Setenv(constants.EnvRPName, rpName)
	for hostPrefix, name := range cookieNames {
		t.Run(name, func(t *testing.T) {
			t.Setenv(dataconstants.EnvSessionCookieHostPrefix, hostPrefix)
			t.Run("passkey", func(t *testing.T) {
				priv := testutil.NewP256Key(t)
				testutil.StubBackend(t, pinnedDirectory(t, priv, telarkconfig.BootstrapStateCompleted))
				rec := httptest.NewRecorder()
				authhandler.LoginFinish(rec, jsonReq(assertionBody(t, priv, flagVerified, signInChallenge(t),
					map[string]any{"email": bootstrapEmail})))
				testutil.Equal(t, "status", rec.Code, http.StatusOK)
				expectSessionCookie(t, rec, name)
			})
			t.Run("google", func(t *testing.T) {
				const email = "test@example.com"
				testutil.StubBackend(t, &callbackExporter{byIdentity: &userresource.User{ID: "u-google", Email: email,
					Status: userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
					Identities: []*userresource.UserIdentity{{Provider: constants.IdentityProviderGoogle,
						Issuer: constants.GoogleIssuer, Subject: callbackSubject}}}})
				rec := httptest.NewRecorder()
				oidchandler.GoogleCallback(rec, jsonReq(signedCallbackBody(t, email)))
				testutil.Equal(t, "status", rec.Code, http.StatusOK)
				expectSessionCookie(t, rec, name)
			})
		})
	}
}

// Logout clears the install's cookie whatever became of the session: valid, unknown or missing.
func TestLogoutClearsTheCookie(t *testing.T) {
	for hostPrefix, name := range cookieNames {
		for state, req := range map[string]*http.Request{
			"session":    withSession(httptest.NewRequest(http.MethodPost, testPath, nil)),
			"no session": httptest.NewRequest(http.MethodPost, testPath, nil),
		} {
			t.Run(name+" "+state, func(t *testing.T) {
				t.Setenv(dataconstants.EnvSessionCookieHostPrefix, hostPrefix)
				rec := httptest.NewRecorder()
				authhandler.Logout(rec, req)
				cookies := rec.Result().Cookies()
				if len(cookies) != oneCookie || cookies[firstCookieAt].Name != name ||
					cookies[firstCookieAt].MaxAge >= firstCookieAt || !cookies[firstCookieAt].Secure ||
					!cookies[firstCookieAt].HttpOnly || cookies[firstCookieAt].Path != "/" {
					t.Fatalf("cookies = %+v, want %s cleared", cookies, name)
				}
			})
		}
	}
}
