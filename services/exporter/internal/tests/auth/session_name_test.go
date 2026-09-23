package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
	"time"

	sessionutil "github.com/telark/exporter/internal/utils/auth/session"
)

const (
	rawToken      = "IoVQ8ZP2rS7m0kQwXn3bYy1cAeF4uJhTgLdNvRp6sKc"
	otherRawToken = "Zk4TmD9wQ1xLpR7nBvC2eH5yU8aJfGsO3dXi6bNtVlM"
	k8sMaxNameLen = 253

	extractSessionSpecErrFmt = "ExtractSessionSpec: %v"
)

var rfc1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func futureTimestamp() string {
	return time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
}

func sessionBody() map[string]any {
	return map[string]any{
		"sessionToken":     rawToken,
		"expiresTimestamp": futureTimestamp(),
	}
}

// The token must never reach etcd. Only its digest does, by way of the
// resource name, and a 256-bit token is not recoverable from that.
func TestSessionNameNeverContainsRawToken(t *testing.T) {
	name := sessionutil.SessionName(rawToken)

	if strings.Contains(name, rawToken) {
		t.Fatalf("session name %q leaks the raw token", name)
	}
}

func TestSessionNameIsTokenDigest(t *testing.T) {
	sum := sha256.Sum256([]byte(rawToken))
	want := "session-" + hex.EncodeToString(sum[:])

	if got := sessionutil.SessionName(rawToken); got != want {
		t.Errorf("SessionName = %q, want %q", got, want)
	}
}

func TestSessionNameIsDeterministic(t *testing.T) {
	first := sessionutil.SessionName(rawToken)
	second := sessionutil.SessionName(rawToken)

	if first != second {
		t.Errorf("SessionName not deterministic: %q then %q", first, second)
	}
}

func TestSessionNameDiffersPerToken(t *testing.T) {
	if sessionutil.SessionName(rawToken) == sessionutil.SessionName(otherRawToken) {
		t.Error("different tokens produced the same session name")
	}
}

func TestSessionNameIsAValidResourceName(t *testing.T) {
	name := sessionutil.SessionName(rawToken)

	if len(name) > k8sMaxNameLen {
		t.Errorf("len(name) = %d, exceeds the %d-character limit", len(name), k8sMaxNameLen)
	}
	if !rfc1123Subdomain.MatchString(name) {
		t.Errorf("name %q is not a valid RFC1123 subdomain", name)
	}
}

// A session spec is what gets persisted, so the token has to be stripped from
// it even though the request body carries one.
func TestExtractSessionSpecStripsTheToken(t *testing.T) {
	session, _, err := sessionutil.ExtractSessionSpec(sessionBody(), testUserID)
	if err != nil {
		t.Fatalf(extractSessionSpecErrFmt, err)
	}
	if session.SessionToken != "" {
		t.Errorf("SessionToken = %q, want it stripped before persistence", session.SessionToken)
	}
}

func TestExtractSessionSpecNamesByDigest(t *testing.T) {
	_, name, err := sessionutil.ExtractSessionSpec(sessionBody(), testUserID)
	if err != nil {
		t.Fatalf(extractSessionSpecErrFmt, err)
	}
	if want := sessionutil.SessionName(rawToken); name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
}

func TestExtractSessionSpecKeepsUserID(t *testing.T) {
	session, _, err := sessionutil.ExtractSessionSpec(sessionBody(), testUserID)
	if err != nil {
		t.Fatalf(extractSessionSpecErrFmt, err)
	}
	if session.UserID != testUserID {
		t.Errorf("UserID = %q, want %q", session.UserID, testUserID)
	}
}
