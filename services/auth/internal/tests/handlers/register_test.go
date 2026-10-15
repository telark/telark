package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

// registerStub backs an account that already holds a passkey: the session
// lookup answers as configured, the passkey list is never empty unless noPasskeys,
// and userDown fails every account lookup.
type registerStub struct {
	session    bool
	noPasskeys bool
	userDown   bool
}

func (s registerStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/api/v1/auth/sessions/self"):
		if !s.session {
			return envelope(http.StatusNotFound, nil)
		}
		return liveSession()
	case strings.Contains(r.URL.Path, "passkeys"):
		if s.noPasskeys {
			return envelope(http.StatusOK, map[string]any{"items": []*authdata.Passkey{}})
		}
		return envelope(http.StatusOK, map[string]any{
			"items": []*authdata.Passkey{{UserID: "uid", CredentialID: "cred"}},
		})
	case s.userDown:
		return envelope(http.StatusInternalServerError, nil)
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
// opening one spends nothing, so the same link can open another after a failed one.
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
	for _, want := range []int{http.StatusOK, http.StatusOK} {
		rec := httptest.NewRecorder()
		passkeyhandler.RegisterStart(rec, jsonReq(body))
		if rec.Code != want {
			t.Fatalf("register/start with enroll token = %d, want %d (body %s)", rec.Code, want, rec.Body.String())
		}
	}
}

// The finish that saves a passkey spends the link before it writes anything, so a failed finish leaves
// it usable. A finish whose link is gone by then (spent by another ceremony, or expired) gets the
// refusal start gives and writes nothing. A ceremony an older pod opened spent its link at start, so it finishes.
func TestEnrollLinkIsSpentByTheSavedPasskey(t *testing.T) {
	initWebAuthn(t)
	cases := []struct {
		name   string
		midway func(t *testing.T, token, challenge string)
		rpID   string
		status int // zero, here and in next: the link refusal
		saved  int
		next   int
	}{
		{"failed finish", nil, otherRPID, http.StatusUnauthorized, constants.DefaultInitValue, http.StatusOK},
		{"saved passkey", nil, webauthnHost, http.StatusCreated, constants.DefaultIncrementValue, constants.DefaultInitValue},
		{"spent by a second ceremony", func(t *testing.T, token, challenge string) {
			testutil.Equal(t, "second finish", finishWith(t, challengeOf(t, startWith(token)), webauthnHost).Code, http.StatusCreated)
			// As if the first finish read its challenge before the second start replaced it.
			if err := redisServer.Set(constants.RedisKeyPrefixChallenge+enrolledTargetID, challenge); err != nil {
				t.Fatalf("restore challenge: %v", err)
			}
		}, webauthnHost, constants.DefaultInitValue, constants.DefaultIncrementValue, constants.DefaultInitValue},
		{"expired mid-ceremony", func(_ *testing.T, token, _ string) {
			redisServer.SetTTL(constants.RedisKeyPrefixEnrollToken+tokenDigest(token), time.Second)
			redisServer.FastForward(2 * time.Second)
		}, webauthnHost, constants.DefaultInitValue, constants.DefaultInitValue, constants.DefaultInitValue},
		{"opened by an older pod", func(t *testing.T, token, challenge string) {
			// It spent the link at start and stored the owner, not a digest.
			redisServer.Del(constants.RedisKeyPrefixEnrollToken + tokenDigest(token))
			if err := redisServer.Set(constants.RedisKeyPrefixEnrolledCeremony+challenge, enrolledTargetID); err != nil {
				t.Fatalf("older ceremony: %v", err)
			}
		}, webauthnHost, http.StatusCreated, constants.DefaultIncrementValue, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			refusal := startWith(unknownToken)
			token, _, err := authhelper.CreateEnrollToken(enrolledTargetID)
			if err != nil {
				t.Fatalf("own link: %v", err)
			}
			challenge := challengeOf(t, startWith(token))
			if c.midway != nil {
				c.midway(t, token, challenge)
			}

			rec := finishWith(t, challenge, c.rpID)
			expectAnswer(t, "finish", rec, c.status, refusal)
			// On top of the passkey the account signed in with to create its link.
			testutil.Equal(t, "passkeys", fake.PasskeyCount(enrolledTargetID), constants.DefaultIncrementValue+c.saved)
			testutil.Equal(t, "identities", len(fake.User(enrolledTargetID).Identities), c.saved)
			expectAnswer(t, "next start", startWith(token), c.next, refusal)
		})
	}
}

// A zero status expects the link refusal itself: the same status and body, so the answers can't be told apart.
func expectAnswer(t *testing.T, step string, rec *httptest.ResponseRecorder, status int, refusal *httptest.ResponseRecorder) {
	t.Helper()
	if status != constants.DefaultInitValue {
		testutil.Equal(t, step, rec.Code, status)
		return
	}
	testutil.Equal(t, step+" status", rec.Code, refusal.Code)
	testutil.Equal(t, step+" answer", rec.Body.String(), refusal.Body.String())
}

// Redis is untrusted, so a link is stored under the digest of its token, never the token.
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const (
	// The account registerStub serves and its session names.
	stubUserID      = "uid"
	unreadableEntry = "not json"
	brokenToken     = "broken-token"
	hashField       = "f"
)

// A finish names its user by the session, or else by the signed ceremony: a pending self-registration,
// an account opened with a link, or one with no passkey yet. A lookup that can't answer fails the finish,
// and an account that holds a passkey gets no other one without a session or a link.
func TestGetUserForRegistrationNamesTheCaller(t *testing.T) {
	cases := []struct {
		name    string
		stub    registerStub
		pending string
		digest  string
		want    string
		refusal string
	}{
		{name: "session", stub: registerStub{session: true}, want: stubUserID},
		{name: "session, account unreadable", stub: registerStub{session: true, userDown: true}},
		{name: "pending entry unreadable", pending: unreadableEntry},
		{name: "account unreadable", stub: registerStub{userDown: true}},
		{name: "account with a passkey", refusal: string(constants.ErrUserAlreadyHasPasskeys)},
		{name: "account opened with a link", digest: hashField, want: stubUserID},
		{name: "account without a passkey", stub: registerStub{noPasskeys: true}, want: stubUserID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stubExporter(t, c.stub)
			if c.pending != constants.EmptyString {
				if err := redisServer.Set(constants.RedisKeyPrefixPendingUser+stubUserID, c.pending); err != nil {
					t.Fatalf("park pending user: %v", err)
				}
				t.Cleanup(func() { redisServer.Del(constants.RedisKeyPrefixPendingUser + stubUserID) })
			}
			r := jsonReq(emptyJSON)
			if c.stub.session {
				r = withSession(r)
			}
			ceremony := func(*http.Request) (string, string, error) { return stubUserID, c.digest, nil }
			_, userID, _, err := authhelper.GetUserForRegistration(r, ceremony)
			if (err == nil) != (c.want != constants.EmptyString) || userID != c.want {
				t.Fatalf("GetUserForRegistration = (%q, %v), want %q", userID, err, c.want)
			}
			if c.refusal != constants.EmptyString {
				testutil.Equal(t, "refusal", err.Error(), c.refusal)
			}
		})
	}
}

// A link store or account lookup that can't answer fails the request: the link is neither honored
// nor reported dead (the 401 every dead link gets), and a finish that can't spend its link saves nothing.
func TestEnrollLinkLookupFaultsFailTheRequest(t *testing.T) {
	initWebAuthn(t)
	fake := inviteDirectory()
	testutil.StubBackend(t, fake)

	// Opening, the link's entry holds another type.
	brokenKey := constants.RedisKeyPrefixEnrollToken + tokenDigest(brokenToken)
	redisServer.HSet(brokenKey, hashField, unreadableEntry)
	t.Cleanup(func() { redisServer.Del(brokenKey) })
	testutil.Equal(t, "start, link unreadable", startWith(brokenToken).Code, http.StatusBadRequest)

	token, _, err := authhelper.CreateEnrollToken(enrolledTargetID)
	if err != nil {
		t.Fatalf("own link: %v", err)
	}
	fake.SetDown(true)
	testutil.Equal(t, "start, account unreadable", startWith(token).Code, http.StatusServiceUnavailable)
	fake.SetDown(false)

	// Finishing, the link's entry turned into another type mid-ceremony.
	challenge := challengeOf(t, startWith(token))
	key := constants.RedisKeyPrefixEnrollToken + tokenDigest(token)
	redisServer.Del(key)
	redisServer.HSet(key, hashField, unreadableEntry)
	t.Cleanup(func() { redisServer.Del(key) })
	testutil.Equal(t, "finish, link unreadable", finishWith(t, challenge, webauthnHost).Code, http.StatusBadRequest)
	testutil.Equal(t, "passkeys", fake.PasskeyCount(enrolledTargetID), constants.DefaultIncrementValue)
}

// The fake exporter turns away an identities patch that doesn't decode, as the exporter does, so no
// test passes on a body the real one would refuse.
func TestFakeExporterRefusesMalformedIdentities(t *testing.T) {
	fake := inviteDirectory()
	rec := httptest.NewRecorder()
	fake.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/v1/users/"+enrolledTargetID,
		strings.NewReader(`{"identities":"`+unreadableEntry+`"}`)))
	testutil.Equal(t, "status", rec.Code, http.StatusBadRequest)
	testutil.Equal(t, "identities", len(fake.User(enrolledTargetID).Identities), constants.DefaultInitValue)
}
