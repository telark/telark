package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/discovery/cache"
	"github.com/telark/discovery/internal/tests/testutil"
)

// The cache key must stay lockstep with the Python enrichment writer.
func TestCacheKey(t *testing.T) {
	testutil.Equal(t, "key", cache.CacheKey("prod", "api"), "enrichment:prod:api")
}

// FlexTime accepts RFC3339 and rejects garbage.
func TestFlexTimeUnmarshal(t *testing.T) {
	var ft cache.FlexTime
	if err := ft.UnmarshalJSON([]byte(`"2026-01-01T00:00:00Z"`)); err != nil {
		t.Fatalf("valid time rejected: %v", err)
	}
	if err := ft.UnmarshalJSON([]byte(`"not-a-time"`)); err == nil {
		t.Fatal("garbage time accepted")
	}
}

// Insights are stale when missing, when the last-updated stamp is unparseable, or
// when enrichment predates the last update; fresh otherwise.
func TestIsStale(t *testing.T) {
	testutil.Equal(t, "nil insights", cache.IsStale(nil, "2026-01-01T00:00:00Z"), true)

	older := "2026-01-01T00:00:00Z"
	newer := "2026-06-01T00:00:00Z"
	stale := &appresource.Insights{EnrichedAt: &older}
	testutil.Equal(t, "enriched before update", cache.IsStale(stale, newer), true)

	fresh := &appresource.Insights{EnrichedAt: &newer}
	testutil.Equal(t, "enriched after update", cache.IsStale(fresh, older), false)

	testutil.Equal(t, "bad last-updated", cache.IsStale(fresh, "garbage"), true)
}

// GetEnrichment tolerates a nil client, a cache miss, and returns parsed insights
// on a hit.
func TestGetEnrichment(t *testing.T) {
	ctx := context.Background()
	if got, err := cache.GetEnrichment(ctx, nil, "prod", "api"); got != nil || err != nil {
		t.Fatalf("nil client = %v, %v", got, err)
	}

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	if got, _ := cache.GetEnrichment(ctx, rdb, "prod", "missing"); got != nil {
		t.Fatal("cache miss should yield nil insights")
	}

	mr.Set(cache.CacheKey("prod", "api"), `{"summary":"web app","enrichedAt":"2026-01-01T00:00:00Z"}`)
	got, err := cache.GetEnrichment(ctx, rdb, "prod", "api")
	if err != nil || got == nil {
		t.Fatalf("cache hit = %v, %v", got, err)
	}
	testutil.Equal(t, "enriched", got.Enriched, true)
}
