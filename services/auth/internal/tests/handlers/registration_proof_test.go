package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
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

	"github.com/fxamacker/cbor/v2"
	"github.com/golang-jwt/jwt/v5"
	roledata "github.com/telark/telark/internal/data/resources/role"
	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	bootstrapEmail      = "admin@example.com"
	newEmail            = "new@example.com"
	webauthnHost        = "example.com"
	webauthnUserPresent = 0x01
	emptyCOSEKey        = 0xA0
	pendingCredID       = "pending-cred"
	trustFileMode       = 0o600
)

// proofStub answers user lookups by email: the bootstrap and existing accounts
// exist, anything else is unknown; passkey lookups fail when asked to.
type proofStub struct {
	passkeyFailure bool
	created        []userresource.User
	deleted        int
}

func (s *proofStub) RoundTrip(r *http.Request) (*http.Response, error) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/api/v1/auth/sessions/self"):
		return envelope(http.StatusNotFound, nil)
	case strings.Contains(r.URL.Path, "passkeys"):
		if s.passkeyFailure {
			return envelope(http.StatusInternalServerError, nil)
		}
		return envelope(http.StatusOK, map[string]any{"items": []any{}})
	case strings.Contains(r.URL.Path, "by-email") && strings.Contains(r.URL.Path, "new"):
		if len(s.created) == constants.DefaultInitValue {
			return envelope(http.StatusNotFound, nil)
		}
		return envelope(http.StatusOK, s.created[constants.DefaultInitValue])
	case r.Method == http.MethodPatch:
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":200}`))}, nil
	case r.Method == http.MethodDelete:
		s.deleted++
		return envelope(http.StatusOK, nil)
	case r.Method == http.MethodPost:
		var user userresource.User
		if err := decodeInto(r, &user); err != nil {
			return nil, err
		}
		user.ID = "u-new"
		s.created = append(s.created, user)
		return envelope(http.StatusCreated, user)
	default:
		return envelope(http.StatusOK, userresource.User{ID: "uid", Email: "a@b.com"})
	}
}

// A bare email may only open a brand-new ReadOnly account: an existing account
// needs a session or an enrollment link, a bootstrap email is reserved for the
// operator, and a passkey lookup that failed never reads as "no passkeys".
func TestRegisterStartProofRules(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, bootstrapEmail)
	t.Setenv(constants.EnvSelfRegistrationEnabled, "true")
	if _, err := config.LoadBootstrapConfig(); err != nil {
		t.Fatalf("LoadBootstrapConfig: %v", err)
	}
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	cases := []struct {
		name           string
		body           string
		passkeyFailure bool
		status         int
	}{
		{"bootstrap email is reserved", `{"email":"` + bootstrapEmail + `"}`, false, http.StatusForbidden},
		{"existing account without passkeys", emailBody, false, http.StatusUnauthorized},
		{"existing account, passkey lookup down", emailBody, true, http.StatusUnauthorized},
		{"unknown email self-registers", `{"email":"` + newEmail + `"}`, false, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &proofStub{passkeyFailure: c.passkeyFailure}
			stubExporter(t, stub)
			rec := httptest.NewRecorder()
			passkeyhandler.RegisterStart(rec, jsonReq(c.body))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
			testutil.Equal(t, "accounts created at start", len(stub.created), constants.DefaultInitValue)
		})
	}
}

// Signs a "none" attestation for the challenge register/start issued, bound to the
// relying party the request host resolves to (example.com).
func registrationBody(t *testing.T, challenge, rpID string) string {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(rpID))
	authData := append(rpIDHash[:], webauthnUserPresent)
	authData = binary.BigEndian.AppendUint32(authData, constants.DefaultIncrementValue)
	authData = append(authData, make([]byte, constants.AAGUIDLength)...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(pendingCredID)))
	authData = append(authData, pendingCredID...)
	authData = append(authData, emptyCOSEKey)
	att, err := cbor.Marshal(map[string]any{"fmt": constants.AttestationFormatNone, "authData": authData})
	if err != nil {
		t.Fatalf("cbor: %v", err)
	}
	clientData, err := json.Marshal(map[string]any{
		"type": constants.WebAuthnTypeCreate, "challenge": challenge, "origin": "http://" + webauthnHost,
	})
	if err != nil {
		t.Fatalf("client data: %v", err)
	}
	id := base64.RawURLEncoding.EncodeToString([]byte(pendingCredID))
	body, err := json.Marshal(map[string]any{"id": id, "rawId": id, "type": "public-key", "response": map[string]any{
		"attestationObject": base64.RawURLEncoding.EncodeToString(att),
		"clientDataJSON":    base64.RawURLEncoding.EncodeToString(clientData),
	}})
	if err != nil {
		t.Fatalf("body: %v", err)
	}
	return string(body)
}

// A self-registration saves its account only when the ceremony finishes with a
// verified credential: a failed finish leaves no account (one whose passkey could
// not be saved is deleted again), a good one creates it ReadOnly.
func TestSelfRegistrationCreatesTheAccountAtFinish(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, bootstrapEmail)
	t.Setenv(constants.EnvSelfRegistrationEnabled, "true")
	if _, err := config.LoadBootstrapConfig(); err != nil {
		t.Fatalf("LoadBootstrapConfig: %v", err)
	}
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
	cases := []struct {
		name           string
		rpID           string
		passkeyFailure bool
		status         int
		created        int
		deleted        int
	}{
		{"credential for another relying party", "evil.example", false, http.StatusUnauthorized,
			constants.DefaultInitValue, constants.DefaultInitValue},
		{"passkey not saved", webauthnHost, true, http.StatusInternalServerError,
			constants.DefaultIncrementValue, constants.DefaultIncrementValue},
		{"verified credential", webauthnHost, false, http.StatusCreated,
			constants.DefaultIncrementValue, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &proofStub{passkeyFailure: c.passkeyFailure}
			stubExporter(t, stub)
			rec := httptest.NewRecorder()
			passkeyhandler.RegisterStart(rec, jsonReq(`{"email":"`+newEmail+`"}`))
			testutil.Equal(t, "start", rec.Code, http.StatusOK)
			var start passkeyhandler.RegisterStartResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
				t.Fatalf("start body: %v", err)
			}

			finish := jsonReq(registrationBody(t, start.Options.Response.Challenge.String(), c.rpID))
			finish.Header.Set(constants.HeaderDeviceName, "laptop")
			finish.Header.Set(constants.HeaderDeviceType, "desktop")
			rec = httptest.NewRecorder()
			passkeyhandler.CreatePasskey(rec, finish)
			testutil.Equal(t, "finish", rec.Code, c.status)
			testutil.Equal(t, "accounts created", len(stub.created), c.created)
			testutil.Equal(t, "accounts deleted", stub.deleted, c.deleted)
			if c.created == c.deleted {
				return
			}
			created := stub.created[constants.DefaultInitValue]
			if created.Email != newEmail || created.Bootstrap || len(created.RoleRefs) != constants.DefaultIncrementValue ||
				*created.RoleRefs[constants.DefaultInitValue] != constants.BuiltInRoleReadOnly {
				t.Fatalf("self-registered user must be ReadOnly without the bootstrap marker, got %+v", created)
			}
		})
	}
}

// The identity-provider trust is changed only by a caller who is Admin on every
// scope; Admin on settings alone is refused before the body is read.
func TestSetConfigNeedsAdminOnAll(t *testing.T) {
	cases := []struct {
		name   string
		grants xauthz.Grants
		status int
	}{
		{"settings admin only", xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeSettings: roledata.PermissionLevelAdmin}}, http.StatusForbidden},
		{"admin on all", xauthz.Grants{Levels: map[string]roledata.PermissionLevel{
			roledata.ScopeAll: roledata.PermissionLevelAdmin}}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: "uid", Grants: c.grants})
			rec := httptest.NewRecorder()
			oidchandler.SetConfig(rec, jsonReq("{").WithContext(ctx))
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, c.status, rec.Body.String())
			}
		})
	}
	if authz.CallerIsAdminOnAll(context.Background()) {
		t.Fatal("no identity must not count as Admin")
	}
}

func decodeInto(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

const (
	callbackClientID = "client"
	callbackKid      = "k-callback"
	callbackSubject  = "sub-callback"
	rsaKeyBits       = 2048
)

// callbackExporter serves what a Google login reads: the OIDC config, the
// subject lookup (bound when byIdentity is set) and the user list an email
// match scans; it counts every write, since a refused login makes none.
type callbackExporter struct {
	byIdentity *userresource.User
	users      []*userresource.User
	mu         sync.Mutex
	writes     int
}

func (e *callbackExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reply := func(status int, data any) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "data": data})
	}
	switch {
	case strings.HasSuffix(r.URL.Path, "/config"):
		reply(http.StatusOK, map[string]any{telarkconfigresource.FieldOIDC: telarkconfigresource.OIDCConfig{
			Enabled: true, GoogleClientID: callbackClientID}})
	case strings.Contains(r.URL.Path, "by-identity") && e.byIdentity == nil:
		reply(http.StatusNotFound, nil)
	case strings.Contains(r.URL.Path, "by-identity"):
		reply(http.StatusOK, e.byIdentity)
	case r.Method == http.MethodGet:
		reply(http.StatusOK, map[string]any{"items": e.users})
	default:
		e.mu.Lock()
		e.writes++
		e.mu.Unlock()
		reply(http.StatusOK, nil)
	}
}

// Signs a Google-shaped ID token for bootstrapEmail with a key the mounted trust file pins.
func signedCallbackBody(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	trust := filepath.Join(t.TempDir(), "googleJwkJson")
	jwks := fmt.Sprintf(`{"keys":[{"kty":"RSA","use":"sig","kid":%q,"alg":"RS256","n":%q,"e":"AQAB"}]}`,
		callbackKid, base64.RawURLEncoding.EncodeToString(key.N.Bytes()))
	if err := os.WriteFile(trust, []byte(jwks), trustFileMode); err != nil {
		t.Fatalf("trust file: %v", err)
	}
	t.Setenv(constants.EnvOIDCTrustFile, trust)
	t.Cleanup(oidchelper.StopJWKSRefresh)

	nonce, err := oidchelper.GenerateAndStoreNonce()
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, oidchelper.GoogleClaims{
		Email: bootstrapEmail, EmailVerified: true, Nonce: nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: constants.GoogleIssuer, Subject: callbackSubject,
			Audience: jwt.ClaimStrings{callbackClientID}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	token.Header[constants.JWTHeaderKid] = callbackKid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return fmt.Sprintf(`{"idToken":%q}`, signed)
}

// The bootstrap admin signs in with a passkey only: a Google login that resolves to
// it, through a subject bound before promotion or through its email, is refused
// with 403 and writes nothing (no identity attached, no session).
func TestGoogleCallbackRefusesTheBootstrapAdmin(t *testing.T) {
	bootstrap := &userresource.User{ID: "u-boot", Email: bootstrapEmail, Bootstrap: true,
		Status: userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		Identities: []*userresource.UserIdentity{{Provider: constants.IdentityProviderGoogle,
			Issuer: constants.GoogleIssuer, Subject: callbackSubject}}}
	cases := []struct {
		name    string
		backend *callbackExporter
	}{
		{"subject bound to the bootstrap admin", &callbackExporter{byIdentity: bootstrap}},
		{"email of the bootstrap admin", &callbackExporter{users: []*userresource.User{{ID: "u-boot", Email: bootstrapEmail, Bootstrap: true}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.StubBackend(t, c.backend)
			rec := httptest.NewRecorder()
			oidchandler.GoogleCallback(rec, jsonReq(signedCallbackBody(t)))
			testutil.Equal(t, "status", rec.Code, http.StatusForbidden)
			testutil.Equal(t, "message", strings.Contains(rec.Body.String(), string(constants.ErrReservedEmail)), true)
			c.backend.mu.Lock()
			defer c.backend.mu.Unlock()
			testutil.Equal(t, "writes", c.backend.writes, constants.DefaultInitValue)
		})
	}
}
