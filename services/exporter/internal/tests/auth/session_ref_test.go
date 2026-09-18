package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	authdata "github.com/telark/data/auth"
	dataconstants "github.com/telark/data/constants"
	"github.com/telark/exporter/internal/constants"
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

func selfRequest(token string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/auth/sessions/tokens/self/get", nil)
	r = mux.SetURLVars(r, map[string]string{constants.TokenParam: authdata.SessionRefSelf})
	if token != "" {
		r.Header.Set(dataconstants.HeaderSessionToken, token)
	}
	return r
}

func TestRefFromRequestResolvesSelfToTheCallerToken(t *testing.T) {
	w := httptest.NewRecorder()
	ref, ok := sessionutil.RefFromRequest(w, selfRequest(rawToken))
	if !ok || ref != rawToken {
		t.Fatalf("self must resolve to the header token, got %q ok=%v", ref, ok)
	}
}

func TestRefFromRequestRejectsSelfWithoutToken(t *testing.T) {
	w := httptest.NewRecorder()
	if _, ok := sessionutil.RefFromRequest(w, selfRequest("")); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("self without a token must be a 400, got ok=%v code=%d", ok, w.Code)
	}
}

func TestRefFromRequestPassesNamesThrough(t *testing.T) {
	name := sessionutil.SessionName(rawToken)
	r := httptest.NewRequest(http.MethodGet, "/auth/sessions/tokens/"+name+"/get", nil)
	r = mux.SetURLVars(r, map[string]string{constants.TokenParam: name})
	if ref, ok := sessionutil.RefFromRequest(httptest.NewRecorder(), r); !ok || ref != name {
		t.Fatalf("a name must pass through, got %q ok=%v", ref, ok)
	}
}

func TestRefFromRequestRefusesARawToken(t *testing.T) {
	r := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{constants.TokenParam: rawToken})
	w := httptest.NewRecorder()

	if _, ok := sessionutil.RefFromRequest(w, r); ok || w.Code != http.StatusBadRequest {
		t.Fatalf("a raw token in the path must be refused with 400, got ok=%v status=%d", ok, w.Code)
	}
	if strings.Contains(w.Body.String(), rawToken) {
		t.Error("the refusal must not echo the token")
	}
}
