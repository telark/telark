package statushandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/handlers/status"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// The Redis client is cached for the process, so the server it binds to outlives every run (-count).
var redisServer *miniredis.Miniredis

func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	redisServer = mr
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	m.Run()
	mr.Close()
}

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

// No exporter answers here, so TelarkConfig never loads: that must surface as
// degraded on a 200, not as the 503 that dropped every replica from the Service
// during the exporter storm. Only Redis going away fails the probe.
func TestReadinessGatesOnRedisOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), config.RedisPingTimeout())
	defer cancel()
	if redishelper.NewRedisClientWithRetry(ctx) == nil {
		t.Fatal("redis client not installed")
	}

	code, data := probeReadiness(t)
	testutil.Equal(t, "status with exporter down", code, http.StatusOK)
	testutil.Equal(t, "degraded", data.Degraded, true)
	testutil.Equal(t, "telarkconfig reason", slices.Contains(data.Reasons, constants.ReadinessReasonTelarkConfig), true)

	redisServer.Close()
	t.Cleanup(func() {
		if err := redisServer.Restart(); err != nil {
			t.Error(err)
		}
	})
	code, data = probeReadiness(t)
	testutil.Equal(t, "status with redis down", code, http.StatusServiceUnavailable)
	testutil.Equal(t, "redis reason", slices.Contains(data.Reasons, constants.ReadinessReasonRedis), true)
}
