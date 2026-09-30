package oidc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	testClientID    = "client"
	rsaModulusBytes = 128
)

// LoadConfig reads the OIDC block from the resource backend; with no backend it
// fails rather than returning a zero config.
func TestLoadConfigFailsClosed(t *testing.T) {
	if _, err := oidc.LoadConfig(); err == nil {
		t.Fatal("LoadConfig should fail with no backend")
	}
}

// Token validation runs entirely against the offline (pasted) key set: an
// unparseable key set is rejected when building the store, and a malformed token
// is rejected during verification. Neither path performs live egress.
func TestValidateGoogleIDTokenOffline(t *testing.T) {
	t.Cleanup(oidc.StopJWKSRefresh)

	badKeys := telarkconfigresource.OIDCConfig{
		Enabled: true, GoogleClientID: testClientID, EgressAllowed: false, GoogleJWKJSON: "not-json",
	}
	if _, err := oidc.ValidateGoogleIDToken("header.claims.sig", badKeys); err == nil {
		t.Fatal("unparseable JWK set should fail store construction")
	}

	emptyKeys := telarkconfigresource.OIDCConfig{
		Enabled: true, GoogleClientID: testClientID, EgressAllowed: false, GoogleJWKJSON: "{}",
	}
	_, err := oidc.ValidateGoogleIDToken("not-a-jwt", emptyKeys)
	testutil.Equal(t, "malformed token rejected", err != nil, true)
}

// A pasted JWK set is parsed into RSA public keys once and reused on the next
// call for the same trust settings; the malformed token still fails to verify.
func TestValidateGoogleIDTokenStaticKeys(t *testing.T) {
	t.Cleanup(oidc.StopJWKSRefresh)

	n := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xAB}, rsaModulusBytes))
	jwks := fmt.Sprintf(
		`{"keys":[{"kty":"RSA","use":"sig","kid":"k1","alg":"RS256","n":%q,"e":"AQAB"}]}`, n)
	cfg := telarkconfigresource.OIDCConfig{
		Enabled: true, GoogleClientID: testClientID, EgressAllowed: false, GoogleJWKJSON: jwks,
	}

	if _, err := oidc.ValidateGoogleIDToken("not-a-jwt", cfg); err == nil {
		t.Fatal("malformed token should fail even with keys loaded")
	}
	if _, err := oidc.ValidateGoogleIDToken("still-not-a-jwt", cfg); err == nil {
		t.Fatal("second call (cached store) should still reject a malformed token")
	}
}
