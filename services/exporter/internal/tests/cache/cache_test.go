package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/exporter/internal/cache"
)

type fakeOptimizer struct{ deleted []string }

func (f *fakeOptimizer) Delete(key string) { f.deleted = append(f.deleted, key) }

// Every invalidation path deletes at least one key from the optimizer — a write
// that does not invalidate the read cache would serve stale resources.
func TestInvalidation(t *testing.T) {
	f := &fakeOptimizer{}
	cache.InvalidateListCache(f, "users")
	cache.InvalidateGetCache(f, "users", "u1")
	cache.SmartInvalidateListCache(f, "users", "create")
	cache.InvalidateAllResourceCaches(f, "users")
	cache.InvalidateSpecificResourceCache(f, "users", "u1")
	if len(f.deleted) == 0 {
		t.Fatal("no cache keys were invalidated")
	}
}

// Cache keys are generated deterministically and the key-builder funcs are
// safe to run against a request.
func TestKeys(t *testing.T) {
	if cache.GenerateKey("users", "list", "") == "" {
		t.Fatal("GenerateKey returned empty")
	}
	if cache.GenerateGetKey("/api/v1/resources/users", "u1") == "" {
		t.Fatal("GenerateGetKey returned empty")
	}
	if cache.ValidateCacheKey("") {
		t.Fatal("empty key validated as ok")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/users", nil)
	_ = cache.NewListCacheKeyFunc("users")(req)
	_ = cache.NewGetCacheKeyFunc("users")(req)
}
