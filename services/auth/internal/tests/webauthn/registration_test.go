package webauthn

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fxamacker/cbor/v2"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	flagUP                = 0x01
	flagUV                = 0x04
	flagBE                = 0x08
	flagBS                = 0x10
	flagAT                = 0x40
	flagED                = 0x80
	coseAlgES256          = -7
	coseAlgES256K         = -47
	credProtectLevel      = 2
	createCeremony        = "webauthn.create"
	getCeremony           = "webauthn.get"
	otherOrigin           = "https://evil.example"
	otherRPID             = "evil.example"
	roundTripUser         = "roundTripUser"
	roundTripCredID       = "round-trip-credential-id"
	otherCredID           = "a-different-credentialid" // as long as roundTripCredID
	registrationChallenge = "registration-challenge-32-bytes!"
	otherChallenge        = "a-challenge-nobody-was-ever-sent"
	loginChallenge        = "login-challenge-of-32-bytes-long"
	malformedCOSEKey      = 0xA0
	finishFailed          = "FinishRegistration = %v"
)

var registrationChallengeB64 = base64.RawURLEncoding.EncodeToString([]byte(registrationChallenge))

func extensionOutputs(t *testing.T) []byte {
	t.Helper()
	ext, err := cbor.Marshal(map[string]any{"credProtect": credProtectLevel})
	if err != nil {
		t.Fatalf("extensions: %v", err)
	}
	return ext
}

// What a browser posts to finish a registration with a "none" attestation; each field
// can be bent away from the valid ceremony to check the library refuses it.
type registration struct {
	flags     byte
	key       []byte
	rpID      string
	origin    string
	ceremony  string
	challenge string
	credID    string
}

func validRegistration(t *testing.T, priv *ecdsa.PrivateKey, flags byte) registration {
	t.Helper()
	return registration{
		flags: flags, key: testutil.COSEKeyEC2(t, &priv.PublicKey, coseAlgES256), rpID: testRPID, origin: testOrigin,
		ceremony: createCeremony, challenge: registrationChallengeB64, credID: roundTripCredID,
	}
}

func (reg registration) body(t *testing.T) []byte {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(reg.rpID))
	authData := append([]byte{}, rpIDHash[:]...)
	authData = append(authData, reg.flags)
	authData = binary.BigEndian.AppendUint32(authData, constants.DefaultInitValue)
	authData = append(authData, make([]byte, aaguidLen)...)
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(roundTripCredID)))
	authData = append(authData, reg.credID...)
	authData = append(authData, reg.key...)
	if reg.flags&flagED != constants.DefaultInitValue {
		authData = append(authData, extensionOutputs(t)...)
	}
	att, err := cbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": authData})
	if err != nil {
		t.Fatalf("attestation: %v", err)
	}
	cd, err := json.Marshal(map[string]any{"type": reg.ceremony, "challenge": reg.challenge, "origin": reg.origin})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	body, err := json.Marshal(map[string]any{
		"id": enc([]byte(roundTripCredID)), "rawId": enc([]byte(roundTripCredID)), "type": "public-key",
		"response": map[string]any{"attestationObject": enc(att), "clientDataJSON": enc(cd)},
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	return body
}

// Starts the ceremony the way StartRegistration leaves it, then finishes it with the body.
func finish(t *testing.T, userID string, reg registration) (*authdata.Passkey, error) {
	t.Helper()
	if err := webauthnhelper.StoreChallenge(userID, registrationChallengeB64); err != nil {
		t.Fatalf(storeChallengeFailed, err)
	}
	r := httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(reg.body(t)))
	cred, elig, state, err := webauthnhelper.FinishRegistration(userID, testName, testDisplay, r)
	if err != nil {
		return nil, err
	}
	return authhelper.CreatePasskeyFromCredential(userID, cred, testName, testName, elig, state), nil
}

// The library decides every registration; each of these was a credential it refused
// while a manual parser still stored it, or a binding that parser also checked.
func TestFinishRegistrationRefuses(t *testing.T) {
	priv := testutil.NewP256Key(t)
	valid := validRegistration(t, priv, flagUP|flagUV|flagAT)
	bend := func(change func(*registration)) registration {
		reg := valid
		change(&reg)
		return reg
	}
	cases := []struct {
		name string
		reg  registration
	}{
		{"malformed public key", bend(func(r *registration) { r.key = []byte{malformedCOSEKey} })},
		{"algorithm not offered", bend(func(r *registration) { r.key = testutil.COSEKeyEC2(t, &priv.PublicKey, coseAlgES256K) })},
		{"other relying party", bend(func(r *registration) { r.rpID = otherRPID })},
		{"origin not allowed", bend(func(r *registration) { r.origin = otherOrigin })},
		{"user not present", bend(func(r *registration) { r.flags = flagUV | flagAT })},
		{"sign-in ceremony", bend(func(r *registration) { r.ceremony = getCeremony })},
		{"challenge never sent", bend(func(r *registration) {
			r.challenge = base64.RawURLEncoding.EncodeToString([]byte(otherChallenge))
		})},
		{"credential id not attested", bend(func(r *registration) { r.credID = otherCredID })},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := finish(t, c.name, c.reg); err == nil {
				t.Fatal("the registration must be refused")
			}
		})
	}
}

// Before the key was split from the extension outputs, the whole authData tail was stored.
func withTrailingExtensions(t *testing.T, passkey *authdata.Passkey) *authdata.Passkey {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(passkey.PublicKey)
	if err != nil {
		t.Fatalf("stored key: %v", err)
	}
	passkey.PublicKey = base64.StdEncoding.EncodeToString(append(key, extensionOutputs(t)...))
	return passkey
}

func assertionRequest(t *testing.T, priv *ecdsa.PrivateKey, flags byte, chal string, userHandle []byte) *http.Request {
	t.Helper()
	rpIDHash := sha256.Sum256([]byte(testRPID))
	authData := binary.BigEndian.AppendUint32(append(append([]byte{}, rpIDHash[:]...), flags), testSignCount)
	cd, err := json.Marshal(map[string]any{"type": getCeremony, "challenge": chal, "origin": testOrigin})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	cdHash := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	body, err := json.Marshal(map[string]any{
		"email": "test@example.com", "id": enc([]byte(roundTripCredID)), "rawId": enc([]byte(roundTripCredID)), "type": "public-key",
		"response": map[string]any{
			"authenticatorData": enc(authData), "clientDataJSON": enc(cd), "signature": enc(sig), "userHandle": enc(userHandle),
		},
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	return httptest.NewRequest(http.MethodPost, testPath, bytes.NewReader(body))
}

// A passkey the library registered must sign in: device-bound, synced (backup
// eligible), one whose authenticator data carried extension outputs after the key,
// and a record stored with those outputs before they were split off.
func TestRegisteredPasskeySignsIn(t *testing.T) {
	cases := []struct {
		name       string
		regFlags   byte
		loginFlags byte
		legacy     bool
	}{
		{"device-bound", flagUP | flagUV | flagAT, flagUP | flagUV, false},
		{"synced", flagUP | flagUV | flagBE | flagBS | flagAT, flagUP | flagUV | flagBE | flagBS, false},
		{"extension outputs after the key", flagUP | flagUV | flagAT | flagED, flagUP | flagUV, false},
		{"record stored with extension outputs", flagUP | flagUV | flagAT, flagUP | flagUV, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			priv := testutil.NewP256Key(t)
			passkey, err := finish(t, roundTripUser, validRegistration(t, priv, c.regFlags))
			if err != nil {
				t.Fatalf(finishFailed, err)
			}
			if c.legacy {
				passkey = withTrailingExtensions(t, passkey)
			}
			passkeys := []*authdata.Passkey{passkey}
			credentials := webauthnhelper.ConvertPasskeysToCredentials(passkeys)
			user := webauthnhelper.CreateUser(roundTripUser, testName, testDisplay, credentials)
			chal := base64.RawURLEncoding.EncodeToString([]byte(loginChallenge))
			r := assertionRequest(t, priv, c.loginFlags, chal, user.ID)

			cred, err := webauthnhelper.VerifyCredential(&authdata.AuthChallenge{Challenge: chal}, user, credentials, passkeys, r)
			if err != nil {
				t.Fatalf("VerifyCredential = %v", err)
			}
			if !bytes.Equal(cred.ID, []byte(roundTripCredID)) {
				t.Fatalf("credential id = %q", cred.ID)
			}
		})
	}
}
