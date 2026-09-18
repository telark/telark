package statushandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/handlers/status"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/discovery/internal/tests/testutil"
)

func probeReadiness(t *testing.T) (int, status.Diagnostics) {
	t.Helper()
	rec := httptest.NewRecorder()
	status.Readiness(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	var body struct {
		Data status.Diagnostics `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return rec.Code, body.Data
}

// No exporter answers here, so GlobalConfig never loads: that must surface as
// degraded on a 200, not as the 503 that dropped every replica from the Service
// during the exporter storm. Only Redis going away fails the probe.
func TestReadinessGatesOnRedisOnly(t *testing.T) {
	mr := testutil.RedisEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), config.RedisPingTimeout())
	defer cancel()
	if redishelper.NewRedisClientWithRetry(ctx) == nil {
		t.Fatal("redis client not installed")
	}

	code, data := probeReadiness(t)
	testutil.Equal(t, "status with exporter down", code, http.StatusOK)
	testutil.Equal(t, "degraded", data.Degraded, true)
	testutil.Equal(t, "globalconfig reason", slices.Contains(data.Reasons, constants.ReadinessReasonGlobalConfig), true)

	mr.Close()
	code, data = probeReadiness(t)
	testutil.Equal(t, "status with redis down", code, http.StatusServiceUnavailable)
	testutil.Equal(t, "redis reason", slices.Contains(data.Reasons, constants.ReadinessReasonRedis), true)
}
