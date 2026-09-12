package auth

import (
	"strings"
	"testing"

	sessionutil "github.com/telark/exporter/internal/utils/auth/session"
)

// The UI can no longer read session tokens out of the list response, so it
// addresses a session by its resource name instead. Both forms must resolve to
// the same record.
func TestResolveSessionRefAcceptsAToken(t *testing.T) {
	if got, want := sessionutil.ResolveSessionRef(rawToken), sessionutil.SessionName(rawToken); got != want {
		t.Errorf("ResolveSessionRef(token) = %q, want %q", got, want)
	}
}

func TestResolveSessionRefAcceptsAName(t *testing.T) {
	name := sessionutil.SessionName(rawToken)

	if got := sessionutil.ResolveSessionRef(name); got != name {
		t.Errorf("ResolveSessionRef(name) = %q, want it unchanged as %q", got, name)
	}
}

// Names and tokens must not be interchangeable on the authentication path: a
// name is public to its owner, a token is a credential. Hashing a name yields a
// different record, so presenting one as a bearer token resolves to nothing.
func TestASessionNameIsNotUsableAsAToken(t *testing.T) {
	name := sessionutil.SessionName(rawToken)

	if sessionutil.SessionName(name) == name {
		t.Fatal("hashing a session name returned the same name; a name could be used as a token")
	}
}

func TestResolveSessionRefRejectsALookalikePrefix(t *testing.T) {
	// Prefixed but not a digest: must be hashed, not trusted as a name.
	ref := "session-not-a-digest"

	if got := sessionutil.ResolveSessionRef(ref); got == ref {
		t.Errorf("ResolveSessionRef(%q) was trusted as a resource name", ref)
	}
}

func TestResolvedNamesStayValidResourceNames(t *testing.T) {
	for _, ref := range []string{rawToken, otherRawToken, sessionutil.SessionName(rawToken)} {
		resolved := sessionutil.ResolveSessionRef(ref)

		if !strings.HasPrefix(resolved, "session-") {
			t.Errorf("ResolveSessionRef(%q) = %q, want the session- prefix", ref, resolved)
		}
		if !rfc1123Subdomain.MatchString(resolved) {
			t.Errorf("ResolveSessionRef(%q) = %q, not a valid resource name", ref, resolved)
		}
	}
}
