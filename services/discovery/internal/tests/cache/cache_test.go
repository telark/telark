package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/discovery/cache"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	docNamespace = "prod"
	docApp       = "api"

	docVersion  = 3
	docSteps    = 4
	docInsights = 1
	docRuns     = 2
)

// The cache key must stay lockstep with the analyzer's document writer.
func TestCacheKey(t *testing.T) {
	testutil.Equal(t, "key", cache.CacheKey(docNamespace, docApp), "analyzer:prod:api")
}

func newRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func TestGetInsightsNilClientAndMiss(t *testing.T) {
	ctx := context.Background()
	if got, err := cache.GetInsights(ctx, nil, docNamespace, docApp); got != nil || err != nil {
		t.Fatalf("nil client = %v, %v", got, err)
	}
	_, rdb := newRedis(t)
	if got, err := cache.GetInsights(ctx, rdb, docNamespace, "missing"); got != nil || err != nil {
		t.Fatalf("redis.Nil = %v, %v", got, err)
	}
}

func TestGetInsightsParsesDocument(t *testing.T) {
	mr, rdb := newRedis(t)
	doc := `{"version":3,"lastRun":{"status":"succeeded","trigger":"manual","runId":"r1","model":"qwen3:4b","steps":4},` +
		`"insights":[{"id":"i1","kind":"incident","title":"crash loop","status":"open","runs":2,` +
		`"evidence":[{"type":"event","ref":"prod/api-1"}]}]}`
	if err := mr.Set(cache.CacheKey(docNamespace, docApp), doc); err != nil {
		t.Fatal(err)
	}
	got, err := cache.GetInsights(context.Background(), rdb, docNamespace, docApp)
	if err != nil || got == nil {
		t.Fatalf("hit = %v, %v", got, err)
	}
	testutil.Equal(t, "version", got.Version, docVersion)
	testutil.Equal(t, "status", got.LastRun.Status, "succeeded")
	testutil.Equal(t, "runId", got.LastRun.RunID, "r1")
	testutil.Equal(t, "steps", got.LastRun.Steps, docSteps)
	testutil.Equal(t, "insights", len(got.Insights), docInsights)
	testutil.Equal(t, "title", got.Insights[0].Title, "crash loop")
	testutil.Equal(t, "runs", got.Insights[0].Runs, docRuns)
	testutil.Equal(t, "evidence", got.Insights[0].Evidence[0].Ref, "prod/api-1")
}

func TestGetInsightsLegacyDocumentIsNil(t *testing.T) {
	mr, rdb := newRedis(t)
	legacy := `{"summary":"web app","enrichedAt":"2026-01-01T00:00:00","tech_stack":["go"]}`
	if err := mr.Set(cache.CacheKey(docNamespace, docApp), legacy); err != nil {
		t.Fatal(err)
	}
	got, err := cache.GetInsights(context.Background(), rdb, docNamespace, docApp)
	if got != nil || err != nil {
		t.Fatalf("legacy = %v, %v", got, err)
	}
}
