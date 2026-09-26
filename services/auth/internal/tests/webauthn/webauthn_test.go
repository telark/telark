package webauthn

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/auth/internal/helpers/webauthn"
	"github.com/telark/auth/internal/tests/testutil"
	authdata "github.com/telark/data/auth"
)

const (
	rpName               = "Test"
	challengeTimeoutSecs = 60
	testPath             = "/"
	testName             = "name"
	testDisplay          = "display"
	testShortName        = "n"
	testShortFull        = "f"
	badBase64Input       = "!!!!"
	testBaseID           = "base"
	testCredID           = "abc"
	testCredB64          = "AQID"
	testRawCredID        = "\x01\x02\x03\x04"
	testChalUser         = "chalUser"
	testPayload          = "payload"
	badBase64Case        = "bad base64"
	jsonMarshalFailed    = "json marshal: %v"
	storeChallengeFailed = "StoreChallenge = %v"
	rpIDHashLen          = 32
	authDataFlagsLen     = rpIDHashLen + 1
	aaguidLen            = 16
	authDataCap          = 64
	tooShortAuthDataLen  = 10
	backupFlagBits       = 0x08 | 0x10
	userPresentBit       = 0x01
	testRPID             = "localhost"
	testOrigin           = "http://localhost:3000"
	testSignCount        = 5
	minimalCOSEKey       = 0xA0
)

var testOrigins = []string{testOrigin}

var pinnedRelyingParty = config.WebAuthnConfig{
	RPID:             "localhost",
	RPName:           rpName,
	RPOrigin:         "http://localhost:3000",
	ChallengeTimeout: challengeTimeoutSecs,
}

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
	if err := webauthnhelper.InitWebAuthn(&pinnedRelyingParty); err != nil {
		panic(err)
	}
	m.Run()
	mr.Close()
}

// The relying-party User adapter must surface exactly the identity fields the
// WebAuthn library asks for.
func TestUserMethods(t *testing.T) {
	u := &webauthnhelper.User{
		ID:          []byte("uid"),
		Name:        testName,
		DisplayName: testDisplay,
		Credentials: []webauthnlib.Credential{{}},
	}
	testutil.Equal(t, "id", string(u.WebAuthnID()), "uid")
	testutil.Equal(t, testName, u.WebAuthnName(), testName)
	testutil.Equal(t, testDisplay, u.WebAuthnDisplayName(), testDisplay)
	testutil.Equal(t, "icon", u.WebAuthnIcon(), constants.EmptyString)
	testutil.Equal(t, "creds", len(u.WebAuthnCredentials()), constants.DefaultIncrementValue)
}

// GetWebAuthnFor returns the pinned instance wired up in TestMain.
func TestGetWebAuthnFor(t *testing.T) {
	wa, err := webauthnhelper.GetWebAuthnFor(httptest.NewRequest(http.MethodPost, testPath, nil))
	if err != nil || wa == nil {
		t.Fatalf("GetWebAuthnFor = (%v, %v), want a live instance", wa, err)
	}
}

// CreateUser stamps a "<baseID>:<suffix>" handle.
func TestCreateUser(t *testing.T) {
	u := webauthnhelper.CreateUser(testBaseID, "uname", "ufull", nil)
	if !strings.HasPrefix(string(u.ID), testBaseID+constants.ColonSeparator) {
		t.Fatalf("handle %q does not carry base id", u.ID)
	}
	testutil.Equal(t, testName, u.Name, "uname")
	testutil.Equal(t, testDisplay, u.DisplayName, "ufull")
}

// Conversion skips nil entries, returns an empty (non-nil) slice for no input,
// and decodes stored credential material back into library credentials.
func TestConvertPasskeysToCredentials(t *testing.T) {
	testutil.Equal(t, "empty", len(webauthnhelper.ConvertPasskeysToCredentials(nil)), constants.DefaultInitValue)
	testutil.Equal(t, "all nil", len(webauthnhelper.ConvertPasskeysToCredentials(
		[]*authdata.UserPasskey{nil, nil})), constants.DefaultInitValue)

	valid := &authdata.UserPasskey{CredentialID: testCredB64, PublicKey: "AAAA"}
	got := webauthnhelper.ConvertPasskeysToCredentials([]*authdata.UserPasskey{nil, valid})
	testutil.Equal(t, "one valid", len(got), constants.DefaultIncrementValue)
}

// The Backup Eligible / Backup State bits are read from a fixed offset in the
// authenticator data; too-short or undecodable input is an error.
func TestExtractBackupFlagsFromAuthenticatorData(t *testing.T) {
	both := make([]byte, authDataFlagsLen)
	both[rpIDHashLen] = backupFlagBits
	none := make([]byte, authDataFlagsLen)

	cases := []struct {
		name      string
		input     string
		wantElig  bool
		wantState bool
		wantErr   bool
	}{
		{"both flags", base64.RawURLEncoding.EncodeToString(both), true, true, false},
		{"no flags", base64.RawURLEncoding.EncodeToString(none), false, false, false},
		{"too short", base64.RawURLEncoding.EncodeToString(make([]byte, tooShortAuthDataLen)), false, false, true},
		{badBase64Case, badBase64Input, false, false, true},
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
	for _, in := range []string{badBase64Input, base64.RawURLEncoding.EncodeToString([]byte("not-cbor"))} {
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
		{CredentialID: testCredID, BackupEligible: true, BackupState: false},
	}
	cases := []struct {
		name    string
		credID  string
		elig    bool
		state   bool
		wantErr bool
	}{
		{"match", testCredID, true, false, false},
		{"eligible mismatch", testCredID, false, false, true},
		{"state mismatch", testCredID, true, true, true},
		{"unknown credential", "zzz", true, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := webauthnhelper.ValidateBackupFlags(c.credID, c.elig, c.state, passkeys)
			testutil.Equal(t, "err", err != nil, c.wantErr)
		})
	}
}

// Backup-flag inconsistencies are recognized by message so login can fall back
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

// The body is drained for inspection but restored so downstream handlers can
// read it again.
func TestReadAndRestoreRequestBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader(testPayload))
	body, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		t.Fatalf("ReadAndRestoreRequestBody = %v", err)
	}
	testutil.Equal(t, "returned body", string(body), testPayload)

	again, err := webauthnhelper.ReadAndRestoreRequestBody(r)
	if err != nil {
		t.Fatalf("second read = %v", err)
	}
	testutil.Equal(t, "restored body", string(again), testPayload)
}

// Session data carries the challenge and the allowed-credential list the library
// verifies an assertion against.
func TestCreateSessionData(t *testing.T) {
	challenge := &authdata.AuthChallenge{Challenge: "chal"}
	user := webauthnhelper.CreateUser("u", testShortName, testShortFull, nil)
	creds := []webauthnlib.Credential{{ID: []byte("\x01\x02")}, {ID: []byte("\x03\x04")}}

	sd := webauthnhelper.CreateSessionData(challenge, user, creds)
	testutil.Equal(t, "challenge", sd.Challenge, "chal")
	testutil.Equal(t, "allowed creds", len(sd.AllowedCredentialIDs), len(creds))
}

// A new challenge supersedes the pending one for the same user, and an unknown
// user is not found.
func TestChallengeLifecycle(t *testing.T) {
	if err := webauthnhelper.StoreChallenge(testChalUser, "value"); err != nil {
		t.Fatalf(storeChallengeFailed, err)
	}

	got, err := webauthnhelper.ValidateAndGetChallenge(testChalUser)
	if err != nil {
		t.Fatalf("ValidateAndGetChallenge = %v", err)
	}
	testutil.Equal(t, "stored challenge", got.Challenge, "value")

	if err := webauthnhelper.StoreChallenge(testChalUser, "other"); err != nil {
		t.Fatalf("second store should supersede the pending ceremony: %v", err)
	}
	got, err = webauthnhelper.ValidateAndGetChallenge(testChalUser)
	if err != nil {
		t.Fatalf("ValidateAndGetChallenge after supersede = %v", err)
	}
	testutil.Equal(t, "superseded challenge", got.Challenge, "other")
	if _, err := webauthnhelper.ValidateAndGetChallenge("missingUser"); err == nil {
		t.Fatal("missing challenge should error")
	}
	webauthnhelper.CleanupChallenge(testChalUser)
}

// StartRegistration issues creation options and persists the challenge for the
// finish step.
func TestStartRegistration(t *testing.T) {
	opts, challenge, err := webauthnhelper.StartRegistration(
		"regUser", testName, "full", nil, httptest.NewRequest(http.MethodPost, testPath, nil))
	if err != nil {
		t.Fatalf("StartRegistration = %v", err)
	}
	if opts == nil || challenge == constants.EmptyString {
		t.Fatal("StartRegistration returned empty options/challenge")
	}
}

// buildAttestation assembles a well-formed "none"-format attestation object,
// matching client data, and credential id — the exact shape the manual parser
// accepts when the library's stricter verification declines.
func buildAttestation(t *testing.T) (attB64, clientDataB64, credIDB64, expectedChallenge string) {
	t.Helper()
	credID := []byte(testRawCredID)

	rpIDHash := sha256.Sum256([]byte(testRPID))
	authData := make([]byte, constants.InitialCapacity, authDataCap)
	authData = append(authData, rpIDHash[:]...)                // rpIdHash
	authData = append(authData, backupFlagBits|userPresentBit) // flags: UP + backup eligible + state
	authData = binary.BigEndian.AppendUint32(authData, testSignCount)
	authData = append(authData, make([]byte, aaguidLen)...) // AAGUID
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(testRawCredID)))
	authData = append(authData, credID...)
	authData = append(authData, minimalCOSEKey)

	attBytes, err := cbor.Marshal(map[string]any{"fmt": "none", "authData": authData})
	if err != nil {
		t.Fatalf("cbor marshal: %v", err)
	}
	attB64 = base64.RawURLEncoding.EncodeToString(attBytes)

	expectedChallenge = base64.RawURLEncoding.EncodeToString([]byte("testchallenge"))
	cd, err := json.Marshal(map[string]any{
		"type":      "webauthn.create",
		"challenge": expectedChallenge,
		"origin":    testOrigin,
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	clientDataB64 = base64.RawURLEncoding.EncodeToString(cd)
	credIDB64 = base64.RawURLEncoding.EncodeToString(credID)
	return attB64, clientDataB64, credIDB64, expectedChallenge
}

// The manual parser reconstructs the credential and its backup flags from a
// valid "none" attestation when the library path is unavailable.
func TestParseAttestationObjectManually(t *testing.T) {
	att, cd, credID, chal := buildAttestation(t)
	cred, elig, state, err := webauthnhelper.ParseAttestationObjectManually(att, cd, credID, chal, testRPID, testOrigins)
	if err != nil || cred == nil {
		t.Fatalf("ParseAttestationObjectManually = (%v, %v)", cred, err)
	}
	testutil.Equal(t, "eligible", elig, true)
	testutil.Equal(t, "state", state, true)
}

// The manual path checks what the library would: the relying party hash, the
// user-present flag, the ceremony type and an allowed origin.
func TestParseAttestationObjectManuallyBinding(t *testing.T) {
	att, cd, credID, chal := buildAttestation(t)
	cases := []struct {
		name    string
		rpID    string
		origins []string
	}{
		{"other relying party", "evil.example", testOrigins},
		{"origin not allowed", testRPID, []string{"https://evil.example"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, _, err := webauthnhelper.ParseAttestationObjectManually(att, cd, credID, chal, c.rpID, c.origins); err == nil {
				t.Fatal("expected a binding error")
			}
		})
	}

	wrongType, err := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": chal, "origin": testOrigin})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	if _, _, _, err := webauthnhelper.ParseAttestationObjectManually(
		att, base64.RawURLEncoding.EncodeToString(wrongType), credID, chal, testRPID, testOrigins); err == nil {
		t.Fatal("a non-registration ceremony type must be rejected")
	}
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
		{badBase64Case, badBase64Input},
		{"not cbor", base64.RawURLEncoding.EncodeToString([]byte("xx"))},
		{"wrong format", base64.RawURLEncoding.EncodeToString(wrongFmt)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, err := webauthnhelper.ParseAttestationObjectManually(
				c.attB64, constants.EmptyString, constants.EmptyString, "chal", testRPID, testOrigins)
			if err == nil {
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
		t.Fatalf(storeChallengeFailed, err)
	}
	body, err := json.Marshal(map[string]any{
		"id":       credID,
		"response": map[string]any{"attestationObject": att, "clientDataJSON": cd},
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}

	r := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
	cred, _, _, err := webauthnhelper.FinishRegistration("finUser", testName, "full", r)
	if err != nil || cred == nil {
		t.Fatalf("FinishRegistration = (%v, %v)", cred, err)
	}

	missing := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader(`{"id":"x"}`))
	if err := webauthnhelper.StoreChallenge("finUser2", chal); err != nil {
		t.Fatalf(storeChallengeFailed, err)
	}
	if _, _, _, err := webauthnhelper.FinishRegistration("finUser2", testShortName, testShortFull, missing); err == nil {
		t.Fatal("registration body without response should fail")
	}
}

// Backup-flag validation over the login body is a no-op for malformed or
// incomplete bodies and rejects a stored/presented flag mismatch.
func TestValidateBackupFlagsFromRequest(t *testing.T) {
	authData := make([]byte, authDataFlagsLen)
	authData[rpIDHashLen] = backupFlagBits
	adB64 := base64.RawURLEncoding.EncodeToString(authData)
	body, err := json.Marshal(map[string]any{
		"id":       testCredB64,
		"response": map[string]any{"authenticatorData": adB64},
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}

	if err := webauthnhelper.ValidateBackupFlagsFromRequest(body, nil); err != nil {
		t.Fatalf("no passkeys should pass: %v", err)
	}
	if err := webauthnhelper.ValidateBackupFlagsFromRequest([]byte("{"), nil); err != nil {
		t.Fatalf("malformed body should be a no-op: %v", err)
	}

	mismatch := []*authdata.UserPasskey{{CredentialID: testCredB64, BackupEligible: false, BackupState: false}}
	if err := webauthnhelper.ValidateBackupFlagsFromRequest(body, mismatch); err == nil {
		t.Fatal("flag mismatch should be rejected")
	}
}

// A malformed assertion body fails verification before any signature check.
func TestVerifyCredentialMalformed(t *testing.T) {
	challenge := &authdata.AuthChallenge{Challenge: "c"}
	user := webauthnhelper.CreateUser("u", testShortName, testShortFull, nil)
	r := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader("{bad"))
	if _, err := webauthnhelper.VerifyCredential(challenge, user, nil, nil, r); err == nil {
		t.Fatal("malformed assertion should fail verification")
	}
}
