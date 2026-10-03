package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"

	dataerrors "github.com/telark/telark/internal/data/errors"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	longLocalPartLen = 100
	separatorLen     = 1
	hexCharsPerByte  = 2
	maxByte          = 255
	byteOne          = 1
	byteTwo          = 2
	byteThree        = 3
	byteFour         = 4
	rootPath         = "/"
	testUserID       = "uid"
	testCredID       = "cred"
	testEmail        = "a@b.com"
	testDeviceName   = "Pixel"
	testDeviceType   = "phone"
)

// A username is the sanitized email local-part plus a random suffix, padded to a
// floor length so it is always a legal handle.
func TestBuildUsername(t *testing.T) {
	cases := []struct {
		name   string
		email  string
		prefix string
	}{
		{"simple", "alice@example.com", "alice_"},
		{"short local", "a@x.com", "a_"},
		{"special chars sanitized", "a.b+c@x.com", "a_b_c_"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := authhelper.BuildUsername(c.email)
			if err != nil {
				t.Fatalf("BuildUsername(%q) = %v", c.email, err)
			}
			if !strings.HasPrefix(got, c.prefix) {
				t.Fatalf("BuildUsername(%q) = %q, want prefix %q", c.email, got, c.prefix)
			}
			if len(got) < constants.UsernameMinLen {
				t.Fatalf("username %q shorter than floor %d", got, constants.UsernameMinLen)
			}
		})
	}

	long := strings.Repeat("x", longLocalPartLen) + "@x.com"
	got, err := authhelper.BuildUsername(long)
	if err != nil {
		t.Fatalf("BuildUsername(long) = %v", err)
	}
	if len(got) > constants.UsernameMaxLocalLen+separatorLen+hexCharsPerByte*constants.UsernameRandomBytes {
		t.Fatalf("local part not truncated: %q", got)
	}
}

// The decoder accepts raw-url tokens and falls back to standard base64 (with
// padding and +/ alphabet) so it tolerates both encodings a browser may send.
func TestDecodeBase64URLWithFallback(t *testing.T) {
	rawBytes := []byte{byteOne, byteTwo, byteThree, byteFour}
	cases := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{"raw url", base64.RawURLEncoding.EncodeToString(rawBytes), rawBytes, false},
		{"std alphabet", "////", []byte{maxByte, maxByte, maxByte}, false},
		{"std padding", "AQ==", []byte{byteOne}, false},
		{"invalid", "!!!", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := authhelper.DecodeBase64URLWithFallback(c.input)
			testutil.Equal(t, "err", err != nil, c.wantErr)
			if !c.wantErr {
				testutil.Equal(t, "decoded", string(got), string(c.want))
			}
		})
	}
}

// Extractors return the header or path value when present and a specific error
// when it is missing; a leftover credential header is not a credential id.
func TestHeaderExtractors(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, rootPath, nil)
	r.Header.Set(constants.HeaderSessionToken, "tok")
	r = testutil.WithCredentialID(r, testCredID)

	tok, err := authhelper.ExtractSessionToken(r)
	testutil.Equal(t, "token", tok, "tok")
	testutil.Equal(t, "token err", err != nil, false)

	cred, err := authhelper.ExtractCredentialID(r)
	testutil.Equal(t, testCredID, cred, testCredID)
	testutil.Equal(t, "cred err", err != nil, false)

	empty := httptest.NewRequest(http.MethodGet, rootPath, nil)
	empty.Header.Set("X-Credential-ID", testCredID)
	if _, err := authhelper.ExtractSessionToken(empty); err == nil {
		t.Fatal("missing session token should error")
	}
	if _, err := authhelper.ExtractCredentialID(empty); err == nil {
		t.Fatal("missing credential id should error")
	}
}

// Both device headers are required together.
func TestValidateDeviceHeaders(t *testing.T) {
	full := httptest.NewRequest(http.MethodGet, rootPath, nil)
	full.Header.Set(constants.HeaderDeviceName, testDeviceName)
	full.Header.Set(constants.HeaderDeviceType, testDeviceType)
	name, typ, err := authhelper.ValidateDeviceHeaders(full)
	if err != nil {
		t.Fatalf("ValidateDeviceHeaders = %v", err)
	}
	testutil.Equal(t, "name", name, testDeviceName)
	testutil.Equal(t, "type", typ, testDeviceType)

	partial := httptest.NewRequest(http.MethodGet, rootPath, nil)
	partial.Header.Set(constants.HeaderDeviceName, testDeviceName)
	if _, _, err := authhelper.ValidateDeviceHeaders(partial); err == nil {
		t.Fatal("missing device type should error")
	}
}

// The exporter's refusal is carried with its status and its message verbatim.
func TestProxyError(t *testing.T) {
	err := &authhelper.ProxyError{Status: http.StatusConflict, Message: "already exists"}
	testutil.Equal(t, "message", err.Error(), "already exists")
}

// An empty token is rejected before any lookup; a real token with no reachable
// backend fails closed as "not found" rather than granting access.
func TestValidateSession(t *testing.T) {
	if _, err := authhelper.ValidateSession(""); err == nil {
		t.Fatal("empty session token should error")
	}

	noHeader := httptest.NewRequest(http.MethodGet, rootPath, nil)
	if _, err := authhelper.ValidateSessionFromRequest(noHeader); err == nil {
		t.Fatal("request without session header should error")
	}

	if _, err := authhelper.ValidateSession("no-such-token"); err == nil {
		t.Fatal("unresolvable session token should fail closed")
	}
}

// The async worker is nil-safe before init, then runs dispatched work to
// completion by the time the drain returns.
func TestAsyncWorker(t *testing.T) {
	authhelper.Dispatch(func() {})
	authhelper.DrainAsyncWorker()

	authhelper.InitAsyncWorker()
	ran := make(chan struct{})
	authhelper.Dispatch(func() { close(ran) })
	authhelper.DrainAsyncWorker()

	select {
	case <-ran:
	default:
		t.Fatal("dispatched work did not run before drain returned")
	}
}

// Every helper that reaches the resource backend fails closed when the backend
// is unreachable rather than returning a partial success.
func TestClientHelpersFailClosed(t *testing.T) {
	if _, _, err := authhelper.GetUserAndPasskeys(testEmail); err == nil {
		t.Fatal("GetUserAndPasskeys should fail with no backend")
	}
	if _, err := authhelper.GetUserByIDWithErrorHandling(testUserID); err == nil {
		t.Fatal("GetUserByIDWithErrorHandling should fail with no backend")
	}
	if _, err := authhelper.CheckUserHasExistingPasskeys(testUserID); err == nil {
		t.Fatal("CheckUserHasExistingPasskeys should fail with no backend")
	}
	if _, err := authhelper.CreatePendingUser(&userresource.User{Email: testEmail}); err == nil {
		t.Fatal("CreatePendingUser should fail with no backend")
	}
	if err := authhelper.UpdatePasskeyLastUsed(testUserID, []byte{1, 2, 3}); err == nil {
		t.Fatal("UpdatePasskeyLastUsed should fail with no backend")
	}
	authhelper.UpdateUserLastLogin(testUserID) // must not panic with no backend
	if err := authhelper.DeletePasskey(testUserID, testCredID, false); err == nil {
		t.Fatal("DeletePasskey should fail with no backend")
	}
}

// Passkey creation surfaces backend failures, and identity attachment reports a
// non-OK patch as an error.
func TestPasskeyMutationsFailClosed(t *testing.T) {
	passkey := authhelper.CreatePasskeyFromCredential(
		testUserID, &webauthnlib.Credential{ID: []byte{1}}, "dev", testDeviceType, false, false)
	if passkey == nil {
		t.Fatal("CreatePasskeyFromCredential returned nil")
	}

	if _, err := authhelper.CreatePasskey(testUserID, passkey); err == nil {
		t.Fatal("CreatePasskey should fail with no backend")
	}
	if _, err := authhelper.UpdatePasskey(testUserID, testCredID, map[string]any{"k": "v"}); err == nil {
		t.Fatal("UpdatePasskey should fail with no backend")
	}

	user := &userresource.User{ID: testUserID}
	if err := authhelper.AttachPasskeyIdentity(testUserID, user, &webauthnlib.Credential{ID: []byte{1}}); err == nil {
		t.Fatal("AttachPasskeyIdentity should fail with no backend")
	}
}

// Registration entry points resolve the caller from the request; with no
// session and an unreachable backend they fail rather than provisioning.
func TestGetUserForRegistrationFailClosed(t *testing.T) {
	byBody := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"email":"a@b.com"}`))
	if _, _, _, err := authhelper.GetUserForRegistrationStart(byBody); err == nil {
		t.Fatal("GetUserForRegistrationStart should fail with no backend")
	}

	noCeremony := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	noCeremony.Header.Set("X-Email", testEmail)
	if _, _, _, err := authhelper.GetUserForRegistration(noCeremony, webauthnhelper.RegistrationChallengeOwner); err == nil {
		t.Fatal("GetUserForRegistration should fail without a registration ceremony")
	}
}

// A session is minted from the cached config; with no backend the create call
// fails, and a returned token is never empty.
func TestCreateUserSession(t *testing.T) {
	t.Setenv("RP_ID", "localhost")
	t.Setenv("RP_NAME", "Test")
	t.Setenv("RP_ORIGIN", "http://localhost:3000")

	token, err := authhelper.CreateUserSession(testUserID, nil)
	if err == nil && token == "" {
		t.Fatal("CreateUserSession returned an empty token without an error")
	}
}

// Every login ends in CreateUserSession, so a user the exporter reports as being
// deleted (410) or whose account is not active is refused a session there, with a
// verdict (403) rather than a fault; a suspended account is told it is suspended.
func TestCreateUserSessionRefusesTerminatingOrInactiveUser(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"terminating", http.StatusGone, `{"status":410,"message":"user is being deleted"}`, string(dataerrors.ErrAuthzUserNotActive)},
		{"inactive", http.StatusOK, `{"data":{"id":"uid","status":{"phase":"inactive"}}}`, string(dataerrors.ErrAuthzUserNotActive)},
		{"suspended", http.StatusOK, `{"data":{"id":"uid","status":{"phase":"suspended"}}}`, "user account is suspended"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			token, err := authhelper.CreateUserSession(testUserID, nil)
			if err == nil || token != "" {
				t.Fatalf("CreateUserSession = (%q, %v), want refusal", token, err)
			}
			testutil.Equal(t, "message", err.Error(), c.want)
			testutil.Equal(t, "status", shared.GetStatusCodeForSessionError(err), http.StatusForbidden)
		})
	}
}

// The last-login stamp names no phase, so the exporter keeps the stored one: a login
// that read the account before an admin suspended it can never switch it back on.
func TestUpdateUserLastLoginLeavesThePhase(t *testing.T) {
	sent := make(chan map[string]map[string]any, constants.DefaultIncrementValue)
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			select {
			case sent <- body:
			default:
			}
		}
		_, _ = w.Write([]byte(`{"status":200}`))
	}))
	authhelper.UpdateUserLastLogin(testUserID)
	var status map[string]any
	select {
	case body := <-sent:
		status = body[constants.UserFieldStatus]
	default:
		t.Fatal("UpdateUserLastLogin sent no patch")
	}
	if _, phaseSent := status["phase"]; phaseSent || status[constants.UserStatusFieldLastLoginAt] == nil {
		t.Fatalf("status patch = %v, want lastLoginAt and no phase", status)
	}
}
