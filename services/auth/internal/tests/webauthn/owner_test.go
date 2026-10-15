package webauthn

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/services/auth/internal/constants"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

var ownerChallengeB64 = base64.RawURLEncoding.EncodeToString([]byte("owner-ceremony-challenge"))

// A registration body as the browser posts it, around the given client data.
func ownerRequest(t *testing.T, clientDataB64 string) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		constants.WebAuthnKeyID: testCredB64,
		constants.WebAuthnKeyResponse: map[string]string{
			constants.WebAuthnKeyAttestationObject: testCredB64,
			constants.WebAuthnKeyClientDataJSON:    clientDataB64,
		},
	})
	if err != nil {
		t.Fatalf(jsonMarshalFailed, err)
	}
	return httptest.NewRequest(http.MethodPost, testPath, strings.NewReader(string(body)))
}

func clientDataWith(challenge string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.create","challenge":"` + challenge + `"}`))
}

// A session-less finish is bound to its ceremony by the challenge the authenticator signed: client
// data or a challenge that doesn't decode, a ceremony nobody opened, or an entry that can't be read
// names no owner.
func TestRegistrationChallengeOwnerRefuses(t *testing.T) {
	rdb := redishelper.GetClient()
	ctx := context.Background()
	ownerKey := constants.RedisKeyPrefixRegistrationOwner + ownerChallengeB64
	enrolledKey := constants.RedisKeyPrefixEnrolledCeremony + ownerChallengeB64
	unreadable := func(key string) func(t *testing.T) {
		return func(t *testing.T) {
			if err := rdb.HSet(ctx, key, testName, testPayload).Err(); err != nil {
				t.Fatalf("hset %s: %v", key, err)
			}
		}
	}
	cases := []struct {
		name       string
		clientData string
		setup      func(t *testing.T)
		want       string
	}{
		{name: "client data not base64", clientData: badBase64Input},
		{name: "client data not JSON", clientData: base64.RawURLEncoding.EncodeToString([]byte(testPayload))},
		{name: "client data without a challenge", clientData: base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.create"}`)),
			want: string(constants.ErrMissingChallengeInClientData)},
		{name: "challenge not base64", clientData: clientDataWith(badBase64Input)},
		{name: "ceremony nobody opened", clientData: clientDataWith(ownerChallengeB64), want: string(constants.ErrChallengeNotFound)},
		{name: "owner entry unreadable", clientData: clientDataWith(ownerChallengeB64), setup: unreadable(ownerKey)},
		{name: "enrolled entry unreadable", clientData: clientDataWith(ownerChallengeB64), setup: func(t *testing.T) {
			if err := webauthnhelper.StoreRegistrationChallengeOwner(ownerChallengeB64, testChalUser); err != nil {
				t.Fatalf("store owner: %v", err)
			}
			unreadable(enrolledKey)(t)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Cleanup(func() { rdb.Del(ctx, ownerKey, enrolledKey) })
			if c.setup != nil {
				c.setup(t)
			}
			userID, _, err := webauthnhelper.RegistrationChallengeOwner(ownerRequest(t, c.clientData))
			if err == nil {
				t.Fatalf("RegistrationChallengeOwner named %q, want a refusal", userID)
			}
			if c.want != constants.EmptyString {
				testutil.Equal(t, "refusal", err.Error(), c.want)
			}
		})
	}
}
