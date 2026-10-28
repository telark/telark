// The cached Redis client is never unset once bound, so the limiter without one gets a package that never binds it.
package noredis

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/ratelimit"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	oneAttempt = "1"
	starts     = 3
)

// Without a Redis client the starts go through past any limit: each then fails at its own write.
func TestFailsOpenWithoutRedis(t *testing.T) {
	t.Setenv(constants.EnvRateLimitPerClient, oneAttempt)
	rt := router.NewRouter([]router.Route{router.CreateRoute(base.Post, autheps.StartLogin,
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })})
	rt.Use(ratelimit.New())
	for range starts {
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, router.Pattern(autheps.StartLogin), nil))
		testutil.Equal(t, "start", rec.Code, http.StatusOK)
	}
}
