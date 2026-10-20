package webauthn

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-webauthn/webauthn/protocol"
	webauthnlib "github.com/go-webauthn/webauthn/webauthn"

	"github.com/alicebob/miniredis/v2"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
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
	retryUser            = "retryUser"
	retryChallenge       = "cmV0cnktY2VyZW1vbnktY2hhbGxlbmdl" // base64url of "retry-ceremony-challenge"
	strayChallenge       = "c3RyYXktY2hhbGxlbmdl"             // base64url of "stray-challenge"
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
	testServiceToken     = "test-service-token"
	testOrigin           = "http://localhost:3000"
	testSignCount        = 5
	minimalCOSEKey       = 0xA0
	// A CBOR map header that promises one entry and holds none.
	truncatedCOSEKeyB64 = "oQ=="
)

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
	// Ceremony entries are signed with it.
	if err := os.Setenv(dataconstants.EnvServiceToken, testServiceToken); err != nil {
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
		[]*authdata.Passkey{nil, nil})), constants.DefaultInitValue)

	valid := &authdata.Passkey{CredentialID: testCredB64, PublicKey: "AAAA"}
	got := webauthnhelper.ConvertPasskeysToCredentials([]*authdata.Passkey{nil, valid})
	testutil.Equal(t, "one valid", len(got), constants.DefaultIncrementValue)

	// A stored key the library cannot read is skipped, never handed over half-parsed.
	truncated := &authdata.Passkey{CredentialID: testCredB64, PublicKey: truncatedCOSEKeyB64}
	testutil.Equal(t, "non-CBOR key skipped", len(webauthnhelper.ConvertPasskeysToCredentials(
		[]*authdata.Passkey{truncated})), constants.DefaultInitValue)
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

// Stored flags must match what the login presents; a mismatch on a known
// credential is rejected, an unknown credential is ignored.
func TestValidateBackupFlags(t *testing.T) {
	passkeys := []*authdata.Passkey{
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

	sd := webauthnhelper.CreateSessionData(challenge, user, creds, protocol.VerificationRequired)
	testutil.Equal(t, "challenge", sd.Challenge, "chal")
	testutil.Equal(t, "allowed creds", len(sd.AllowedCredentialIDs), len(creds))
	testutil.Equal(t, "user verification", sd.UserVerification, protocol.VerificationRequired)
}

// The body of a finish whose client data names challenge.
func echoing(t *testing.T, challenge string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{constants.WebAuthnKeyResponse: map[string]string{
		constants.WebAuthnKeyClientDataJSON: clientDataWith(challenge)}})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	return body
}

// Redis holds only a challenge's digest, as the name of the ceremony's own key. Another start for the account leaves
// the first ceremony open, a stray finish spends nothing, a finish spends the ceremony its challenge names once, and
// an unknown user has none.
func TestChallengeLifecycle(t *testing.T) {
	store := func(challenge string) {
		t.Helper()
		if err := webauthnhelper.StoreChallenge(testChalUser, challenge); err != nil {
			t.Fatalf(storeChallengeFailed, err)
		}
	}
	consume := func(userID, challenge string) error {
		_, err := webauthnhelper.ValidateAndGetChallenge(userID, echoing(t, challenge))
		return err
	}
	key := func(challenge string) string {
		return constants.RedisKeyPrefixChallenge + testChalUser + constants.ColonSeparator + testutil.ChallengeDigest(t, challenge)
	}

	store(registrationChallengeB64)
	if stored, err := redishelper.GetClient().Get(context.Background(), key(registrationChallengeB64)).Result(); err != nil ||
		stored != constants.RedisChallengeEntry {
		t.Fatalf("stored %q (%v), want an entry named by the challenge's digest", stored, err)
	}
	got, err := webauthnhelper.ValidateAndGetChallenge(testChalUser, echoing(t, registrationChallengeB64))
	if err != nil {
		t.Fatalf("ValidateAndGetChallenge = %v", err)
	}
	testutil.Equal(t, "challenge", got.Challenge, registrationChallengeB64)

	store(registrationChallengeB64)
	store(retryChallenge)
	if consume(testChalUser, strayChallenge) == nil {
		t.Fatal("a finish naming no open ceremony counted")
	}
	if err := consume(testChalUser, registrationChallengeB64); err != nil {
		t.Fatalf("a second start or a stray finish closed the first ceremony: %v", err)
	}
	if consume(testChalUser, registrationChallengeB64) == nil {
		t.Fatal("a spent ceremony finished again")
	}
	if err := consume(testChalUser, retryChallenge); err != nil {
		t.Fatalf("the other ceremony = %v", err)
	}
	if consume("missingUser", retryChallenge) == nil {
		t.Fatal("missing challenge should error")
	}
	if webauthnhelper.StoreChallenge(testChalUser, badBase64Input) == nil {
		t.Fatal("a challenge that doesn't decode was stored")
	}
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

// FinishRegistration hands the body to the library and returns the credential with
// the backup flags and the bare key it verified; a body without a response fails.
func TestFinishRegistration(t *testing.T) {
	reg := validRegistration(t, testutil.NewP256Key(t), flagUP|flagUV|flagAT|flagBE|flagBS|flagED)
	passkey, err := finish(t, "finUser", reg)
	if err != nil {
		t.Fatalf(finishFailed, err)
	}
	testutil.Equal(t, "eligible", passkey.BackupEligible, true)
	testutil.Equal(t, "state", passkey.BackupState, true)
	// The library re-encodes the key, so it is compared decoded: the same key, nothing after it.
	stored, err := base64.StdEncoding.DecodeString(passkey.PublicKey)
	if err != nil {
		t.Fatalf("stored key: %v", err)
	}
	var got, want map[int]any
	rest, err := cbor.UnmarshalFirst(stored, &got)
	if err != nil || len(rest) != constants.DefaultInitValue {
		t.Fatalf("stored key = %d trailing bytes, err %v", len(rest), err)
	}
	if err := cbor.Unmarshal(reg.key, &want); err != nil {
		t.Fatalf("sent key: %v", err)
	}
	testutil.Equal(t, "stored key", fmt.Sprint(got), fmt.Sprint(want))

	if err := webauthnhelper.StoreChallenge("finUser2", registrationChallengeB64); err != nil {
		t.Fatalf(storeChallengeFailed, err)
	}
	missing := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader(`{"id":"x"}`))
	if _, _, _, err := webauthnhelper.FinishRegistration("finUser2", testShortName, testShortFull, missing); err == nil {
		t.Fatal("registration body without response should fail")
	}
}

// Stores a retry's challenge when the finish first reads its body, after it consumed its own.
type retryOnRead struct {
	io.Reader
	once  sync.Once
	retry func()
}

func (b *retryOnRead) Read(p []byte) (int, error) {
	b.once.Do(b.retry)
	return b.Reader.Read(p)
}

// The finish consumes its own challenge; its later cleanup, keyed by the user alone,
// wiped the challenge a quick retry had just started with.
func TestFinishKeepsTheChallengeOfARetry(t *testing.T) {
	if err := webauthnhelper.StoreChallenge(retryUser, registrationChallengeB64); err != nil {
		t.Fatalf(storeChallengeFailed, err)
	}
	body := validRegistration(t, testutil.NewP256Key(t), flagUP|flagUV|flagAT).body(t)
	reader := &retryOnRead{Reader: bytes.NewReader(body), retry: func() {
		if err := webauthnhelper.StoreChallenge(retryUser, retryChallenge); err != nil {
			t.Errorf(storeChallengeFailed, err)
		}
	}}

	r := httptest.NewRequest(http.MethodPost, testPath, reader)
	if _, _, _, err := webauthnhelper.FinishRegistration(retryUser, testName, testShortFull, r); err != nil {
		t.Fatalf("FinishRegistration = %v", err)
	}
	got, err := webauthnhelper.ValidateAndGetChallenge(retryUser, echoing(t, retryChallenge))
	if err != nil {
		t.Fatalf("retry's challenge gone: %v", err)
	}
	testutil.Equal(t, "retry's challenge", got.Challenge, retryChallenge)
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

	mismatch := []*authdata.Passkey{{CredentialID: testCredB64, BackupEligible: false, BackupState: false}}
	if err := webauthnhelper.ValidateBackupFlagsFromRequest(body, mismatch); err == nil {
		t.Fatal("flag mismatch should be rejected")
	}
}

// A malformed assertion body fails verification before any signature check.
func TestVerifyCredentialMalformed(t *testing.T) {
	challenge := &authdata.AuthChallenge{Challenge: "c"}
	user := webauthnhelper.CreateUser("u", testShortName, testShortFull, nil)
	r := httptest.NewRequest(http.MethodPost, testPath, strings.NewReader("{bad"))
	if _, err := webauthnhelper.VerifyCredential(challenge, user, nil, nil, r, protocol.VerificationPreferred); err == nil {
		t.Fatal("malformed assertion should fail verification")
	}
}
