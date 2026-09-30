package shared

import (
	"encoding/base64"
	"regexp"
	"testing"

	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

const (
	sessionTokenBytes   = 32
	uniqueTokenSamples  = 100
	generateTokenFailed = "GenerateSessionToken: %v"
)

var base64URLAlphabet = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Only the digest of a session token is persisted, so the token itself has to
// carry enough entropy that the digest cannot be worked backwards.
func TestSessionTokenCarries256Bits(t *testing.T) {
	token, err := shared.GenerateSessionToken()
	if err != nil {
		t.Fatalf(generateTokenFailed, err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not base64url: %v", err)
	}
	if len(raw) != sessionTokenBytes {
		t.Errorf("token decodes to %d bytes, want %d", len(raw), sessionTokenBytes)
	}
}

func TestSessionTokenUsesBase64URLAlphabet(t *testing.T) {
	token, err := shared.GenerateSessionToken()
	if err != nil {
		t.Fatalf(generateTokenFailed, err)
	}

	if !base64URLAlphabet.MatchString(token) {
		t.Errorf("token %q leaves the base64url alphabet", token)
	}
}

func TestSessionTokensAreUnique(t *testing.T) {
	seen := make(map[string]struct{})

	for range uniqueTokenSamples {
		token, err := shared.GenerateSessionToken()
		if err != nil {
			t.Fatalf(generateTokenFailed, err)
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("duplicate session token generated: %q", token)
		}
		seen[token] = struct{}{}
	}
}
