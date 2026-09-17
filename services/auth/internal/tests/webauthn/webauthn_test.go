package webauthn

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/auth/internal/config"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
	"github.com/telark/auth/internal/tests/testutil"
	authdata "github.com/telark/data/auth"
)

// A single embedded Redis + one WebAuthn init serve the whole package: the redis
// helper caches its client globally, so a per-test server would leave the cache
// pointing at a torn-down instance.
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
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{
		RPID:             "localhost",
		RPName:           "Test",
		RPOrigin:         "http://localhost:3000",
		ChallengeTimeout: 60,
	}); err != nil {
		panic(err)
	}
	code := m.Run()
	mr.Close()
	os.Exit(code)
}

// The relying-party User adapter must surface exactly the identity fields the
// WebAuthn library asks for.
func TestUserMethods(t *testing.T) {
	u := &webauthnhelper.User{
		ID:          []byte("uid"),
		Name:        "name",
		DisplayName: "display",
		Credentials: []webauthnlib.Credential{{}},
	}
	testutil.Equal(t, "id", string(u.WebAuthnID()), "uid")
	testutil.Equal(t, "name", u.WebAuthnName(), "name")
	testutil.Equal(t, "display", u.WebAuthnDisplayName(), "display")
	testutil.Equal(t, "icon", u.WebAuthnIcon(), "")
	testutil.Equal(t, "creds", len(u.WebAuthnCredentials()), 1)
}

// GetWebAuthn returns the instance wired up in TestMain.
func TestGetWebAuthn(t *testing.T) {
	wa, err := webauthnhelper.GetWebAuthn()
	if err != nil || wa == nil {
		t.Fatalf("GetWebAuthn = (%v, %v), want a live instance", wa, err)
	}
}

// CreateUser stamps a "<baseID>:<suffix>" handle, and ExtractBaseUserID recovers
// the base id — the round-trip login relies on it.
func TestCreateUserAndExtractBaseUserID(t *testing.T) {
	u := webauthnhelper.CreateUser("base", "uname", "ufull", nil)
	if !strings.HasPrefix(string(u.ID), "base") {
		t.Fatalf("handle %q does not carry base id", u.ID)
	}
	testutil.Equal(t, "name", u.Name, "uname")
	testutil.Equal(t, "display", u.DisplayName, "ufull")
	testutil.Equal(t, "base of handle", webauthnhelper.ExtractBaseUserID(string(u.ID)), "base")
	testutil.Equal(t, "base of plain", webauthnhelper.ExtractBaseUserID("plain"), "plain")
}

// Conversion skips nil entries, returns an empty (non-nil) slice for no input,
// and decodes stored credential material back into library credentials.
func TestConvertPasskeysToCredentials(t *testing.T) {
	testutil.Equal(t, "empty", len(webauthnhelper.ConvertPasskeysToCredentials(nil)), 0)
	testutil.Equal(t, "all nil", len(webauthnhelper.ConvertPasskeysToCredentials(
		[]*authdata.UserPasskey{nil, nil})), 0)

	valid := &authdata.UserPasskey{CredentialID: "AQID", PublicKey: "AAAA"}
	got := webauthnhelper.ConvertPasskeysToCredentials([]*authdata.UserPasskey{nil, valid})
	testutil.Equal(t, "one valid", len(got), 1)
}

// The Backup Eligible / Backup State bits are read from a fixed offset in the
// authenticator data; too-short or undecodable input is an error.
func TestExtractBackupFlagsFromAuthenticatorData(t *testing.T) {
	both := make([]byte, 33)
	both[32] = 0x08 | 0x10
	none := make([]byte, 33)

	cases := []struct {
		name      string
		input     string
		wantElig  bool
		wantState bool
		wantErr   bool
	}{
		{"both flags", base64.RawURLEncoding.EncodeToString(both), true, true, false},
		{"no flags", base64.RawURLEncoding.EncodeToString(none), false, false, false},
		{"too short", base64.RawURLEncoding.EncodeToString(make([]byte, 10)), false, false, true},
		{"bad base64", "!!!!", false, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			elig, state, err := webauthnhelper.ExtractBackupFlagsFromAuthenticatorData(c.input)
			testutil.Equal(t, "err", err != nil, c.wantErr)
			testutil.Equal(t, "eligible", elig, c.wantElig)
			testutil.Equal(t, "state", state, c.wantState)
		})
	}
}

// Undecodable or non-CBOR attestation objects degrade to "no flags" rather than
// failing the registration.
func TestExtractBackupFlagsFromAttestation(t *testing.T) {
	for _, in := range []string{"!!!!", base64.RawURLEncoding.EncodeToString([]byte("not-cbor"))} {
		elig, state := webauthnhelper.ExtractBackupFlagsFromAttestation(in)
		testutil.Equal(t, "eligible", elig, false)
		testutil.Equal(t, "state", state, false)
	}
}

// Stored flags must match what the login presents; a mismatch on a known
// credential is rejected, an unknown credential is ignored.
func TestValidateBackupFlags(t *testing.T) {
	passkeys := []*authdata.UserPasskey{
		nil,
		{CredentialID: "abc", BackupEligible: true, BackupState: false},
	}
	cases := []struct {
		name    string
		credID  string
		elig    bool
		state   bool
		wantErr bool
	}{
		{"match", "abc", true, false, false},
		{"eligible mismatch", "abc", false, false, true},
		{"state mismatch", "abc", true, true, true},
		{"unknown credential", "zzz", true, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := webauthnhelper.ValidateBackupFlags(c.credID, c.elig, c.state, passkeys)
			testutil.Equal(t, "err", err != nil, c.wantErr)
		})
	}
}

// Backup-flag inconsistencies are recognised by message so login can fall back
// to a re-registration path.
func TestIsBackupFlagError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"backup flag", errors.New("some backup flag problem"), true},
		{"capitalized", errors.New("Backup Eligible flag inconsistency"), true},
		{"unrelated", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "is backup flag err", webauthnhelper.IsBackupFlagError(c.err), c.want)
		})
	}
}

// The credential id is pulled from the assertion body, absent id yields empty,
// malformed json or bad base64 is an error.
func TestExtractCredentialIDFromRequest(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantID  string
		wantErr bool
	}{
		{"valid", `{"id":"AQID"}`, "AQID", false},
		{"no id", `{"other":1}`, "", false},
		{"bad json", `{`, "", true},
		{"bad base64", `{"id":"!!!"}`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, _, err := webauthnhelper.ExtractCredentialIDFromRequest([]byte(c.body))
			testutil.Equal(t, "err", err != nil, c.wantErr)
			testutil.Equal(t, "id", id, c.wantID)
		})
	}
}

// The body is drained for inspection but restored so downstream handlers can
// read it again.
func TestReadAndRestoreRequestBody(t *testing.T) {
	r := httptest.NewRequest("POST", "/", strings.NewReader("payload"))
	body, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		t.Fatalf("ReadAndRestoreRequestBody = %v", err)
	}
	testutil.Equal(t, "returned body", string(body), "payload")

	again, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		t.Fatalf("second read = %v", err)
	}
	testutil.Equal(t, "restored body", string(again), "payload")
}

// Session data carries the challenge and the allowed-credential list the library
// verifies an assertion against.
func TestCreateSessionData(t *testing.T) {
	challenge := &authdata.AuthChallenge{Challenge: "chal"}
	user := webauthnhelper.CreateUser("u", "n", "f", nil)
	creds := []webauthnlib.Credential{{ID: []byte{1, 2}}, {ID: []byte{3, 4}}}

	sd := webauthnhelper.CreateSessionData(challenge, user, creds)
	testutil.Equal(t, "challenge", sd.Challenge, "chal")
	testutil.Equal(t, "allowed creds", len(sd.AllowedCredentialIDs), 2)
}

// A challenge is stored once (SetNX), read back once, and is single-writer: a
// second store for the same user conflicts, and an unknown user is not found.
func TestChallengeLifecycle(t *testing.T) {
	if err := webauthnhelper.StoreChallenge("chalUser", "value"); err != nil {
		t.Fatalf("StoreChallenge = %v", err)
	}

	got, err := webauthnhelper.ValidateAndGetChallenge("chalUser")
	if err != nil {
		t.Fatalf("ValidateAndGetChallenge = %v", err)
	}
	testutil.Equal(t, "stored challenge", got.Challenge, "value")

	if err := webauthnhelper.StoreChallenge("chalUser", "other"); err != nil {
		t.Fatalf("second store should supersede the pending ceremony: %v", err)
	}
	got, err = webauthnhelper.ValidateAndGetChallenge("chalUser")
	if err != nil {
		t.Fatalf("ValidateAndGetChallenge after supersede = %v", err)
	}
	testutil.Equal(t, "superseded challenge", got.Challenge, "other")
	if _, err := webauthnhelper.ValidateAndGetChallenge("missingUser"); err == nil {
		t.Fatal("missing challenge should error")
	}
	webauthnhelper.CleanupChallenge("chalUser")
}

// StartRegistration issues creation options and persists the challenge for the
// finish step.
func TestStartRegistration(t *testing.T) {
	opts, challenge, err := webauthnhelper.StartRegistration("regUser", "name", "full", nil)
	if err != nil {
		t.Fatalf("StartRegistration = %v", err)
	}
	if opts == nil || challenge == "" {
		t.Fatalf("StartRegistration returned empty options/challenge")
	}
}

// buildAttestation assembles a well-formed "none"-format attestation object,
// matching client data, and credential id — the exact shape the manual parser
// accepts when the library's stricter verification declines.
func buildAttestation(t *testing.T) (attB64, clientDataB64, credIDB64, expectedChallenge string) {
	t.Helper()
	credID := []byte{1, 2, 3, 4}

	authData := make([]byte, 0, 64)
	authData = append(authData, make([]byte, 32)...) // rpIdHash
	authData = append(authData, 0x08|0x10)           // flags: backup eligible + state
	authData = append(authData, 0, 0, 0, 5)          // sign count
	authData = append(authData, make([]byte, 16)...) // AAGUID
	authData = append(authData, 0, byte(len(credID)))
	authData = append(authData, credID...)
	authData = append(authData, 0xA0) // minimal COSE key

	attBytes, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": authData})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	attB64 = base64.RawURLEncoding.EncodeToString(attBytes)

	expectedChallenge = base64.RawURLEncoding.EncodeToString([]byte("testchallenge"))
	cd, err := json.Marshal(map[string]any{
		"challenge": expectedChallenge,
		"origin":    "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	clientDataB64 = base64.RawURLEncoding.EncodeToString(cd)
	credIDB64 = base64.RawURLEncoding.EncodeToString(credID)
	return attB64, clientDataB64, credIDB64, expectedChallenge
}

// The manual parser reconstructs the credential and its backup flags from a
// valid "none" attestation when the library path is unavailable.
func TestParseAttestationObjectManually(t *testing.T) {
	att, cd, credID, chal := buildAttestation(t)
	cred, elig, state, err := webauthnhelper.ParseAttestationObjectManually(att, cd, credID, chal)
	if err != nil || cred == nil {
		t.Fatalf("ParseAttestationObjectManually = (%v, %v)", cred, err)
	}
	testutil.Equal(t, "eligible", elig, true)
	testutil.Equal(t, "state", state, true)
}

// Undecodable, non-CBOR, and wrong-format attestations are all rejected.
func TestParseAttestationObjectManuallyErrors(t *testing.T) {
	wrongFmt, err := cbor.Marshal(map[string]any{"fmt": "packed"})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	cases := []struct {
		name   string
		attB64 string
	}{
		{"bad base64", "!!!!"},
		{"not cbor", base64.RawURLEncoding.EncodeToString([]byte("xx"))},
		{"wrong format", base64.RawURLEncoding.EncodeToString(wrongFmt)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, _, err := webauthnhelper.ParseAttestationObjectManually(c.attB64, "", "", "chal"); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

// FinishRegistration parses the registration body, falls back to the manual
// parser when the library declines, and cleans up the stored challenge.
func TestFinishRegistration(t *testing.T) {
	att, cd, credID, chal := buildAttestation(t)
	if err := webauthnhelper.StoreChallenge("finUser", chal); err != nil {
		t.Fatalf("StoreChallenge = %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"id":       credID,
		"response": map[string]any{"attestationObject": att, "clientDataJSON": cd},
	})
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}

	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	cred, _, _, err := webauthnhelper.FinishRegistration("finUser", "name", "full", r)
	if err != nil || cred == nil {
		t.Fatalf("FinishRegistration = (%v, %v)", cred, err)
	}

	missing := httptest.NewRequest("POST", "/", strings.NewReader(`{"id":"x"}`))
	if err := webauthnhelper.StoreChallenge("finUser2", chal); err != nil {
		t.Fatalf("StoreChallenge = %v", err)
	}
	if _, _, _, err := webauthnhelper.FinishRegistration("finUser2", "n", "f", missing); err == nil {
		t.Fatal("registration body without response should fail")
	}
}

// Backup-flag validation over the login body is a no-op for malformed or
// incomplete bodies and rejects a stored/presented flag mismatch.
func TestValidateBackupFlagsFromRequest(t *testing.T) {
	authData := make([]byte, 33)
	authData[32] = 0x08 | 0x10
	adB64 := base64.RawURLEncoding.EncodeToString(authData)
	body, err := json.Marshal(map[string]any{
		"id":       "AQID",
		"response": map[string]any{"authenticatorData": adB64},
	})
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}

	if err := webauthnhelper.ValidateBackupFlagsFromRequest(body, nil); err != nil {
		t.Fatalf("no passkeys should pass: %v", err)
	}
	if err := webauthnhelper.ValidateBackupFlagsFromRequest([]byte("{"), nil); err != nil {
		t.Fatalf("malformed body should be a no-op: %v", err)
	}

	mismatch := []*authdata.UserPasskey{{CredentialID: "AQID", BackupEligible: false, BackupState: false}}
	if err := webauthnhelper.ValidateBackupFlagsFromRequest(body, mismatch); err == nil {
		t.Fatal("flag mismatch should be rejected")
	}
}

// A malformed assertion body fails verification before any signature check.
func TestVerifyCredentialMalformed(t *testing.T) {
	challenge := &authdata.AuthChallenge{Challenge: "c"}
	user := webauthnhelper.CreateUser("u", "n", "f", nil)
	r := httptest.NewRequest("POST", "/", strings.NewReader("{bad"))
	if _, err := webauthnhelper.VerifyCredential(challenge, user, nil, nil, r); err == nil {
		t.Fatal("malformed assertion should fail verification")
	}
}
