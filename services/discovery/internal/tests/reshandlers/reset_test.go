package reshandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	"github.com/telark/discovery/internal/handlers/resources/applications"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/discovery/internal/tests/testutil"
)

const siblingApp = shopApp + "-2"

func resetApplication(name string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	applications.ResetApplication(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: name}))
	return rec
}

func jsonResponse(t *testing.T, status int, payload any) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
}

// The exporter host is a fixed cluster DNS name, so its answers are served by
// swapping the default transport the rest client dials through.
func stubExporter(t *testing.T, answer func(*http.Request) *http.Response) {
	t.Helper()
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) { return answer(r), nil })
	t.Cleanup(func() { http.DefaultTransport = prev })
}

func storedAnswer(t *testing.T, app applicationmodel.Application) map[string]any {
	t.Helper()
	return map[string]any{"status": http.StatusOK, "data": app}
}

// The exporter serves the stored app but refuses the CRD delete, so the reset
// fails with 502 after purging Redis. The cooldown armed before the attempt must
// be gone afterwards, or the operator's retry would come back 200 without
// deleting anything. Keys of an app whose name merely starts with the reset
// app's name stay: the old `<app>*` patterns wiped them too.
func TestResetApplicationFailureClearsCooldown(t *testing.T) {
	mr := testutil.RedisEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), constants.AppResetHandlerTimeout)
	defer cancel()
	if redishelper.NewRedisClientWithRetry(ctx) == nil {
		t.Fatal("redis client not installed")
	}
	stubExporter(t, func(r *http.Request) *http.Response {
		if r.Method == http.MethodDelete {
			return jsonResponse(t, http.StatusBadGateway, map[string]any{"status": http.StatusBadGateway, "message": "exporter down"})
		}
		return jsonResponse(t, http.StatusOK, storedAnswer(t, applicationmodel.Application{Name: shopApp}))
	})
	// Left behind, the floor defers the reset app's first flushes as stale for
	// up to 10 min and the pending set is reused for a generation it no longer has.
	stale := []string{
		constants.KeyPrefixHistoryFloor + shopApp,
		constants.KeyPrefixSnapshotPending + shopApp,
		constants.KeyPrefixHistoryPost + shopApp,
		constants.KeyPrefixHistoryRecorded + shopApp,
		constants.KeyPrefixCoalesceBuffer + shopApp,
		constants.KeyPrefixIncidentState + shopApp,
		constants.KeyPrefixGraceScale + shopApp,
		constants.KeyPrefixLockApp + shopApp,
	}
	sibling := []string{
		constants.KeyPrefixIncidentState + siblingApp,
		constants.KeyPrefixGraceScale + siblingApp,
		constants.KeyPrefixLockApp + siblingApp,
		constants.ForceSyncStateKeyPrefix + siblingApp,
	}
	for _, key := range append(stale, sibling...) {
		if err := mr.Set(key, "1"); err != nil {
			t.Fatal(err)
		}
	}

	testutil.Equal(t, "first reset", resetApplication(shopApp).Code, http.StatusBadGateway)
	testutil.Equal(t, "cooldown cleared", mr.Exists(constants.KeyPrefixResetCooldown+shopApp), false)
	for _, key := range stale {
		testutil.Equal(t, key+" purged", mr.Exists(key), false)
	}
	for _, key := range sibling {
		testutil.Equal(t, key+" kept", mr.Exists(key), true)
	}
	testutil.Equal(t, "retry after failure", resetApplication(shopApp).Code, http.StatusBadGateway)
}

// Resetting or syncing an application the store does not know answered 200 and 500.
func TestResetAndSyncOfUnknownApplicationAre404(t *testing.T) {
	stubExporter(t, func(*http.Request) *http.Response {
		return jsonResponse(t, http.StatusNotFound, map[string]any{"status": http.StatusNotFound, "message": "not found"})
	})
	testutil.Equal(t, "reset", resetApplication("nope").Code, http.StatusNotFound)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	applications.SyncApplication(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: "nope"}))
	testutil.Equal(t, "sync", rec.Code, http.StatusNotFound)
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
	got, err := applications.LeaderResetURL(ctx, rdb, leaderID, shopApp)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "forward url", got, "http://10.0.1.7:8080/api/v1/resources/applications/shop/reset")

	_, err = applications.LeaderResetURL(ctx, rdb, "telark-discovery-service-5d6c4d9d58-zzzzz", shopApp)
	testutil.Equal(t, "unknown leader rejected", err != nil, true)
}
