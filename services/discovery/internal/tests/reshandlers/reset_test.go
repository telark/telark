package reshandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
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
	// Left behind, the floor defers the reset app's first flushes as stale for
	// up to 10 min and the pending set is reused for a generation it no longer has.
	stale := []string{
		constants.KeyPrefixHistoryFloor + "shop",
		constants.KeyPrefixSnapshotPending + "shop",
		constants.KeyPrefixHistoryPost + "shop",
		constants.KeyPrefixHistoryRecorded + "shop",
		constants.KeyPrefixCoalesceBuffer + "shop",
	}
	for _, key := range stale {
		if err := mr.Set(key, "1"); err != nil {
			t.Fatal(err)
		}
	}

	testutil.Equal(t, "first reset", resetApplication("shop").Code, http.StatusBadGateway)
	testutil.Equal(t, "cooldown cleared", mr.Exists(constants.KeyPrefixResetCooldown+"shop"), false)
	for _, key := range stale {
		testutil.Equal(t, key+" purged", mr.Exists(key), false)
	}
	testutil.Equal(t, "retry after failure", resetApplication("shop").Code, http.StatusBadGateway)
}

// A leader ID is a pod name, which no DNS resolves behind a ClusterIP Service;
// the forward must dial the address the leader advertised with its heartbeat.
func TestLeaderResetURLUsesAdvertisedAddress(t *testing.T) {
	mr := testutil.RedisEnv(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()
	ctx := context.Background()
	leaderID := "telark-discovery-service-5d6c4d9d58-b27wn"

	coordination.AdvertiseReplica(ctx, rdb, leaderID, "10.0.1.7")
	got, err := applications.LeaderResetURL(ctx, rdb, leaderID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "forward url", got, "http://10.0.1.7:8080/api/v1/resources/applications/shop/reset")

	_, err = applications.LeaderResetURL(ctx, rdb, "telark-discovery-service-5d6c4d9d58-zzzzz", "shop")
	testutil.Equal(t, "unknown leader rejected", err != nil, true)
}
