package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdata "github.com/telark/telark/internal/data/auth"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

// registerStub backs an account that already holds a passkey: the session
// lookup answers as configured, the passkey list is never empty.
type registerStub struct {
	session bool
}

func (s registerStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/api/v1/auth/sessions/self"):
		if !s.session {
			return envelope(http.StatusNotFound, nil)
		}
		return liveSession()
	case strings.Contains(r.URL.Path, "passkeys"):
		return envelope(http.StatusOK, map[string]any{
			"items": []*authdata.Passkey{{UserID: "uid", CredentialID: "cred"}},
		})
	default:
		return envelope(http.StatusOK, userresource.User{ID: "uid", Email: "a@b.com"})
	}
}

// A signed-in user opens a registration for a second passkey through its
// session, naming its own email or none; naming another account is refused,
// and without a session an account that already has passkeys is turned away.
func TestRegisterStartForExistingUser(t *testing.T) {
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	cases := []struct {
		name    string
		session bool
		body    string
		status  int
		code    string
	}{
		{"session without email", true, "", http.StatusOK, constants.EmptyString},
		{"session with own email", true, `{"email":"a@b.com"}`, http.StatusOK, constants.EmptyString},
		{"session naming another email", true, `{"email":"x@y.com"}`, http.StatusForbidden, "email_mismatch"},
		{"no session", false, `{"email":"a@b.com"}`, http.StatusUnauthorized, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubExporter(t, registerStub{session: c.session})
			r := jsonReq(c.body)
			if c.session {
				r.Header.Set(constants.HeaderSessionToken, "token")
			}
			rec := httptest.NewRecorder()
			passkeyhandler.RegisterStart(rec, r)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			testutil.Equal(t, "code", refusalCode(t, rec), c.code)
		})
	}
}

// A signed-in user mints a one-time enrollment link, stored only as its digest;
// presenting its token opens a registration for that user without a session, and
// the token dies on first use so the same link cannot open a second ceremony.
func TestEnrollLinkRegisterStart(t *testing.T) {
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	stubExporter(t, registerStub{session: true})

	rec := httptest.NewRecorder()
	passkeyhandler.CreateEnrollLink(rec, withSession(httptest.NewRequest(http.MethodPost, "/", nil)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll-link status = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var link passkeyhandler.EnrollLinkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatalf("decode enroll-link: %v", err)
	}
	if link.Token == constants.EmptyString || link.ExpiresAt == constants.EmptyString {
		t.Fatalf("enroll-link body = %+v, want a token and an expiry", link)
	}
	if owner, err := redisServer.Get(constants.RedisKeyPrefixEnrollToken + tokenDigest(link.Token)); err != nil || owner != "uid" {
		t.Fatalf("enroll token under its digest = %q (%v), want the signed-in user", owner, err)
	}
	expectNoRawToken(t, link.Token)

	body := `{"enrollToken":"` + link.Token + `"}`
	for _, want := range []int{http.StatusOK, http.StatusUnauthorized} {
		rec := httptest.NewRecorder()
		passkeyhandler.RegisterStart(rec, jsonReq(body))
		if rec.Code != want {
			t.Fatalf("register/start with enroll token = %d, want %d (body %s)", rec.Code, want, rec.Body.String())
		}
	}
}

// Redis is untrusted, so a link is stored under the digest of its token, never the token.
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
