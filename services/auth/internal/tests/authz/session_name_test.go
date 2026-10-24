package authz

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	liveToken  = "live-session-token"
	liveUserID = "u-1"
)

// The exporter knows a session only by the name the token hashes to.
func sessionByName(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(dataconstants.HeaderSessionToken) != authdata.SessionName(liveToken) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"status": http.StatusOK, "data": map[string]any{
		"userId": liveUserID, "expiresTimestamp": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}})
}

// A session's name, which its owner can read (GET auth/sessions, GET self), is no credential: presented as
// X-Session-Token it is not found, which x-ware answers with 401.
func TestSessionNameIsNoCredential(t *testing.T) {
	testutil.StubBackend(t, http.HandlerFunc(sessionByName))
	resolver := authz.NewResolver()
	if user, err := resolver.UserIDForToken(liveToken); err != nil || user != liveUserID {
		t.Fatalf("token = (%q, %v), want %s", user, err, liveUserID)
	}
	if _, err := resolver.UserIDForToken(authdata.SessionName(liveToken)); !errors.Is(err, xauthz.ErrNotFound) {
		t.Fatalf("the session's name = %v, want not found", err)
	}
}
