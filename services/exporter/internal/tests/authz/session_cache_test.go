package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/data/constants"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/performance"
	authendpoints "github.com/telark/rest/endpoints/auth"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
)

const primedSessions = `[{"userId":"u-10000-0000-0001","ipAddress":"203.0.113.7"}]`

// A list primed by its owner used to be served from the path-keyed cache before
// the owner check, so any session could read another user's devices and IPs.
func TestSessionListIsNotServedFromCacheToAnotherUser(t *testing.T) {
	mr := miniredis.RunT(t)
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(optimizer.Close)

	path := pathFor(authendpoints.GetAllSessionsByUser)
	keyed := mux.SetURLVars(httptest.NewRequest(http.MethodGet, path, nil), map[string]string{constants.UserIDParam: subjectUserID})
	key := cache.NewSubjectListCacheKeyFunc(optimizer, constants.ResourceUserSession, cache.SubjectFromPathParam(constants.UserIDParam))(keyed)
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
