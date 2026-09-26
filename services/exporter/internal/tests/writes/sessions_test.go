package writes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	authdata "github.com/telark/data/auth"
	dataconstants "github.com/telark/data/constants"
	"github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	otherUserID    = "u-00002-0000-0002"
	ownerToken     = "owner-token"
	secondToken    = "second-token"
	otherToken     = "other-token"
	missingToken   = "missing-token"
	sessionsPrefix = "/api/v1/auth/sessions/"
)

func sessionSeed(token, owner string) seed {
	md := v1alpha1.SessionMetadata
	return crSeed(md, object(md, authdata.SessionName(token), map[string]any{"userId": owner}, nil))
}

func deleteRequest(name string, identity xauthz.Identity) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, sessionsPrefix+name, nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

// Revoking another device is limited to the caller's own sessions; someone
// else's reads as missing so the route cannot probe for names.
func TestGuardOwnSessionName(t *testing.T) {
	installFake(t, sessionSeed(ownerToken, userID), sessionSeed(otherToken, otherUserID))
	owner := xauthz.Identity{UserID: userID}

	cases := []struct {
		name     string
		session  string
		identity xauthz.Identity
		want     bool
		code     int
	}{
		{"own session", authdata.SessionName(ownerToken), owner, true, http.StatusOK},
		{"another account's session", authdata.SessionName(otherToken), owner, false, http.StatusNotFound},
		{"missing session", authdata.SessionName(missingToken), owner, false, http.StatusNotFound},
		{"internal caller", authdata.SessionName(otherToken), xauthz.Identity{Internal: true}, true, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardOwnSessionName(w, deleteRequest(tc.session, tc.identity), tc.session); got != tc.want || w.Code != tc.code {
				t.Fatalf("GuardOwnSessionName = %v (%d), want %v (%d)", got, w.Code, tc.want, tc.code)
			}
		})
	}
}

// "self" also fits {name}; the self route is registered first so it keeps it,
// and anything that is not a session name is refused before any lookup.
func TestDeleteSessionByNameRouting(t *testing.T) {
	client := installFake(t, sessionSeed(ownerToken, userID), sessionSeed(secondToken, userID), sessionSeed(otherToken, otherUserID))
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	rt := router.NewRouter(routes.InitRoutes(optimizer))
	owner := xauthz.Identity{UserID: userID}

	notAName := httptest.NewRecorder()
	rt.ServeHTTP(notAName, deleteRequest(otherToken, owner))
	if notAName.Code != http.StatusBadRequest {
		t.Errorf("raw token in the path = %d, want %d", notAName.Code, http.StatusBadRequest)
	}

	foreign := httptest.NewRecorder()
	rt.ServeHTTP(foreign, deleteRequest(authdata.SessionName(otherToken), owner))
	if foreign.Code != http.StatusNotFound {
		t.Errorf("another account's session = %d, want %d", foreign.Code, http.StatusNotFound)
	}

	own := httptest.NewRecorder()
	rt.ServeHTTP(own, deleteRequest(authdata.SessionName(ownerToken), owner))
	if own.Code != http.StatusOK {
		t.Fatalf("own session = %d, want %d: %s", own.Code, http.StatusOK, own.Body.String())
	}
	for _, w := range writes(t, client) {
		t.Errorf("revoke wrote instead of deleting: %v", w.body)
	}

	selfRequest := deleteRequest(authdata.SessionRefSelf, owner)
	selfRequest.Header.Set(dataconstants.HeaderSessionToken, secondToken)
	self := httptest.NewRecorder()
	rt.ServeHTTP(self, selfRequest)
	if self.Code != http.StatusOK {
		t.Errorf("self with X-Session-Token = %d, want the self route's %d (the {name} route refuses self)", self.Code, http.StatusOK)
	}
}
