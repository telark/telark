package auth

import (
	"strings"
	"testing"

	"github.com/telark/data/auth"
)

const rawToken = "iTHIL0HaTt08283iQEWPnhQ5b2x80_KcT8t1c_DTuRk"

func TestSessionRefNeverExposesTheToken(t *testing.T) {
	ref := auth.SessionRef(rawToken)
	if strings.Contains(ref, rawToken) || !auth.IsSessionName(ref) {
		t.Fatalf("SessionRef(token) = %q, want a session name", ref)
	}
	if ref != auth.SessionName(rawToken) {
		t.Fatal("SessionRef must hash exactly like SessionName")
	}
}

func TestSessionRefPassesNamesAndSelfThrough(t *testing.T) {
	name := auth.SessionName(rawToken)
	if auth.SessionRef(name) != name {
		t.Fatal("a session name must pass through unchanged")
	}
	if auth.SessionRef(auth.SessionRefSelf) != auth.SessionRefSelf {
		t.Fatal("the self ref must pass through unchanged")
	}
	if auth.IsSessionName(auth.SessionNamePrefix + "zz") {
		t.Fatal("a malformed digest must not count as a name")
	}
}
