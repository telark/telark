package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	authendpoints "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

const primedSessions = `[{"userId":"u-10000-0000-0001","ipAddress":"203.0.113.7"}]`

// A list primed by its owner used to be served from the subject-keyed cache before
// the owner check, so any session could read another user's devices and IPs.
func TestSessionListIsNotServedFromCacheToAnotherUser(t *testing.T) {
	mr := miniredis.RunT(t)
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(optimizer.Close)

	path := pathFor(authendpoints.GetAllSessionsByUser) + "?" + authendpoints.QuerySessionUser + "=" + subjectUserID
	keyed := httptest.NewRequest(http.MethodGet, path, nil)
	key := cache.NewQueryListCacheKeyFunc(optimizer, constants.ResourceUserSession, authendpoints.QuerySessionUser)(keyed)
	optimizer.Set(key, primedSessions)

	middleware, err := xauthz.New(xauthz.Config{
		Resolver:     stubResolver{},
		Requirements: authz.Requirements(),
		RouteKey:     router.KeyFromRequest,
		ServiceToken: testServiceToken,
	})
	if err != nil {
		t.Fatalf("building middleware: %v", err)
	}
	rt := router.NewRouter(routes.InitRoutes(optimizer))
	rt.Use(middleware)

	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set(dataconstants.HeaderSessionToken, sessionOfOutsider)
	recorder := httptest.NewRecorder()
	rt.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden || strings.Contains(recorder.Body.String(), "203.0.113.7") {
		t.Fatalf("outsider got %d %q, want 403 without the primed list", recorder.Code, recorder.Body.String())
	}
}
