package reshandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/handlers/resources/applications"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/discovery/internal/tests/testutil"
)

func resetApplication(name string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	applications.ResetApplication(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: name}))
	return rec
}

// No exporter answers here, so every reset fails at the CRD delete with 502.
// The cooldown armed before the attempt must be gone afterwards, or the
// operator's retry would come back 200 without deleting anything.
func TestResetApplicationFailureClearsCooldown(t *testing.T) {
	mr := testutil.RedisEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), constants.AppResetHandlerTimeout)
	defer cancel()
	if redishelper.NewRedisClientWithRetry(ctx) == nil {
		t.Fatal("redis client not installed")
	}

	testutil.Equal(t, "first reset", resetApplication("shop").Code, http.StatusBadGateway)
	testutil.Equal(t, "cooldown cleared", mr.Exists(constants.KeyPrefixResetCooldown+"shop"), false)
	testutil.Equal(t, "retry after failure", resetApplication("shop").Code, http.StatusBadGateway)
}
