package shared

import (
	"regexp"
	"testing"

	"github.com/telark/auth/internal/helpers/shared"
)

const sessionTokenBytes = 32

var base64URLAlphabet = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Only the digest of a session token is persisted, so the token itself has to
// carry enough entropy that the digest cannot be worked backwards.
func TestSessionTokenCarries256Bits(t *testing.T) {
	token, err := shared.GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	raw, err := shared.Base64URLDecode(token)
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
		t.Fatalf("GenerateSessionToken: %v", err)
	}

	if !base64URLAlphabet.MatchString(token) {
		t.Errorf("token %q leaves the base64url alphabet", token)
	}
}

func TestSessionTokensAreUnique(t *testing.T) {
	seen := make(map[string]struct{})

	for range 100 {
		token, err := shared.GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken: %v", err)
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("duplicate session token generated: %q", token)
		}
		seen[token] = struct{}{}
	}
}
