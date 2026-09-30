package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	authendpoints "github.com/telark/telark/internal/rest/endpoints/auth"
	sessionutil "github.com/telark/telark/services/exporter/internal/utils/auth/session"
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

func selfRequest(header string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions/self", nil)
	if header != "" {
		r.Header.Set(dataconstants.HeaderSessionToken, header)
	}
	return r
}

// The UI sends its raw token; the name is what every lookup uses.
func TestRefFromRequestResolvesARawTokenToItsName(t *testing.T) {
	w := httptest.NewRecorder()
	ref, ok := sessionutil.RefFromRequest(w, selfRequest(rawToken))
	if !ok || ref != sessionutil.SessionName(rawToken) {
		t.Fatalf("a raw token must resolve to its session name, got %q ok=%v", ref, ok)
	}
}

func TestRefFromRequestRejectsAMissingHeader(t *testing.T) {
	w := httptest.NewRecorder()
	if _, ok := sessionutil.RefFromRequest(w, selfRequest("")); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("no X-Session-Token must be a 400, got ok=%v code=%d", ok, w.Code)
	}
}

// Peers (the auth service) send the session name, never the token.
func TestRefFromRequestPassesNamesThrough(t *testing.T) {
	name := sessionutil.SessionName(rawToken)
	if ref, ok := sessionutil.RefFromRequest(httptest.NewRecorder(), selfRequest(name)); !ok || ref != name {
		t.Fatalf("a name must pass through, got %q ok=%v", ref, ok)
	}
}

func TestRefFromRequestRefusesTheLiteralSelf(t *testing.T) {
	w := httptest.NewRecorder()
	if _, ok := sessionutil.RefFromRequest(w, selfRequest(authdata.SessionRefSelf)); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("a literal self header must be refused with 400, got ok=%v status=%d", ok, w.Code)
	}
}

// The path never carries a session ref, so the token cannot reach an access log.
func TestSelfSessionRoutesCarryNoTokenInThePath(t *testing.T) {
	for _, endpoint := range []string{
		string(authendpoints.GetSelfSession), string(authendpoints.PatchSelfSession), string(authendpoints.DeleteSelfSession),
	} {
		if strings.Contains(endpoint, "{") {
			t.Errorf("self session route %q takes a path parameter", endpoint)
		}
	}
}
