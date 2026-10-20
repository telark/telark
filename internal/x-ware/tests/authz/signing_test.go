package authz

import (
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/x-ware/authz"
)

const (
	signedBinding  = "auth:passkey:invite:digest"
	signedPayload  = `{"scopes":{"settings":"owner"},"rules":{}}`
	signingToken   = "test-service-token"
	signFailed     = "SignRedisEntry failed with a service token present"
	tamperedSuffix = "x"
)

func TestSignedEntryRoundTrips(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, signingToken)

	signed, ok := authz.SignRedisEntry(signedBinding, signedPayload)
	if !ok {
		t.Fatal(signFailed)
	}

	payload, ok := authz.VerifyRedisEntry(signedBinding, signed)
	if !ok {
		t.Fatal("a freshly signed entry failed verification")
	}
	if payload != signedPayload {
		t.Errorf("payload = %q, want %q", payload, signedPayload)
	}
}

func TestTamperedEntryIsRejected(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, signingToken)

	signed, ok := authz.SignRedisEntry(signedBinding, signedPayload)
	if !ok {
		t.Fatal(signFailed)
	}

	if _, ok := authz.VerifyRedisEntry(signedBinding, signed+tamperedSuffix); ok {
		t.Error("a tampered entry was accepted")
	}
}

// A signature is bound to the key it was issued for, so an entry cannot be
// lifted from one key to another.
func TestSignatureIsBoundToTheKey(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, signingToken)

	signed, ok := authz.SignRedisEntry(signedBinding, signedPayload)
	if !ok {
		t.Fatal(signFailed)
	}

	if _, ok := authz.VerifyRedisEntry(signedBinding+tamperedSuffix, signed); ok {
		t.Error("an entry signed for one key verified for another")
	}
}

// With no signing key there is nothing to verify against, so an entry must be
// treated as untrusted rather than trusted blindly.
func TestUnsignableEntriesAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, dataconstants.EmptyString)

	if _, ok := authz.SignRedisEntry(signedBinding, signedPayload); ok {
		t.Error("signing succeeded without a service token")
	}
	if _, ok := authz.VerifyRedisEntry(signedBinding, signedPayload); ok {
		t.Error("verification succeeded without a service token")
	}
}
