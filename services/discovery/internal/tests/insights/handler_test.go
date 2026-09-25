package insights_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/cache"
	"github.com/telark/discovery/internal/handlers/insights"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	visibleNS     = "prod"
	visibleApp    = "api"
	visibleKey    = visibleNS + "/" + visibleApp
	hiddenNS      = "kube-system"
	hiddenApp     = "coredns"
	hiddenKey     = hiddenNS + "/" + hiddenApp
	storedVersion = 7
	storedDoc     = `{"version":7,"lastRun":{"status":"done","trigger":"manual","runId":"r1"},"insights":[]}`

	idHiddenCard   = "hidden-card"
	idVisibleCard  = "visible-card"
	paramNamespace = "namespace"
	oneCard        = 1
)

// The discovery Redis helper installs one process-wide client, so every test
// in this package shares the server that client dialed.
var mr *miniredis.Miniredis

func TestMain(m *testing.M) {
	mr = miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), constants.InsightsReadTimeout)
	if redishelper.NewRedisClientWithRetry(ctx) == nil {
		panic("redis client not installed")
	}
	cancel()
	m.Run()
	mr.Close()
}

func reset(t *testing.T, excluded []string) {
	t.Helper()
	mr.FlushAll()
	gcfghelper.SetExcludedForTest(excluded)
}

func read(t *testing.T, apps string) insights.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	insights.GetApplicationsInsights(rec, httptest.NewRequest(http.MethodGet, "/?apps="+apps, http.NoBody))
	testutil.Equal(t, "status", rec.Code, http.StatusOK)
	var body struct {
		Data insights.Response `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Data
}

func store(t *testing.T, namespace, name string) {
	t.Helper()
	if err := mr.Set(cache.CacheKey(namespace, name), storedDoc); err != nil {
		t.Fatal(err)
	}
}

// An excluded namespace is invisible to the whole product: its key must not
// surface as a result, nor as pending (which would promise a later answer).
func TestReadDropsExcludedNamespaces(t *testing.T) {
	reset(t, []string{hiddenNS})
	store(t, hiddenNS, hiddenApp)
	got := read(t, hiddenKey+","+visibleKey)
	_, inResults := got.Results[hiddenKey]
	testutil.Equal(t, "excluded in results", inResults, false)
	testutil.Equal(t, "excluded in pending", slices.Contains(got.Pending, hiddenKey), false)
	testutil.Equal(t, "visible still pending", slices.Contains(got.Pending, visibleKey), true)
}

func TestReadPendingWhenMissing(t *testing.T) {
	reset(t, nil)
	got := read(t, visibleKey)
	testutil.Equal(t, "results", len(got.Results), constants.DefaultInitValue)
	testutil.Equal(t, "pending", slices.Equal(got.Pending, []string{visibleKey}), true)
}

// A multi-namespace app's card about a workload in an excluded namespace must not leak through its visible document.
func TestReadDropsCardsInExcludedWorkloadNamespaces(t *testing.T) {
	reset(t, []string{hiddenNS})
	raw, err := json.Marshal(application.AppInsights{
		Version: storedVersion,
		LastRun: application.LastRun{Status: application.RunStatusDone},
		Insights: []application.Insight{
			{ID: idHiddenCard, Params: map[string]string{paramNamespace: hiddenNS}},
			{ID: idVisibleCard},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mr.Set(cache.CacheKey(visibleNS, visibleApp), string(raw)); err != nil {
		t.Fatal(err)
	}
	doc := read(t, visibleKey).Results[visibleKey]
	testutil.Equal(t, "cards left", len(doc.Insights), oneCard)
	testutil.Equal(t, "visible card kept", doc.Insights[constants.DefaultInitValue].ID, idVisibleCard)
	testutil.Equal(t, "version unchanged", doc.Version, storedVersion)
}

func TestReadReturnsDocument(t *testing.T) {
	reset(t, nil)
	store(t, visibleNS, visibleApp)
	got := read(t, visibleKey)
	doc, ok := got.Results[visibleKey]
	testutil.Equal(t, "in results", ok, true)
	testutil.Equal(t, "version", doc.Version, storedVersion)
	testutil.Equal(t, "pending", len(got.Pending), constants.DefaultInitValue)
}
