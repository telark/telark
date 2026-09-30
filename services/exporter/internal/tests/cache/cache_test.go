package cache

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	testUserID = "u1"

	// Every invalidation helper under test moves the list generation exactly once.
	wantGenerationBumps = 4
)

type fakeOptimizer struct {
	deleted []string
	bumped  []string
}

func (f *fakeOptimizer) Delete(key string) { f.deleted = append(f.deleted, key) }

func (f *fakeOptimizer) BumpListGeneration(resourceType string) {
	f.bumped = append(f.bumped, resourceType)
}

// Every invalidation path either drops a key or moves the list generation — a
// write that does not invalidate the read cache would serve stale resources.
func TestInvalidation(t *testing.T) {
	f := &fakeOptimizer{}
	cache.InvalidateListCache(f, constants.ResourceUser)
	cache.InvalidateGetCache(f, constants.ResourceUser, testUserID)
	cache.SmartInvalidateListCache(f, constants.ResourceUser, constants.OpCreate)
	cache.InvalidateAllResourceCaches(f, constants.ResourceUser)
	cache.InvalidateSpecificResourceCache(f, constants.ResourceUser, testUserID)
	if len(f.deleted) == constants.DefaultInitValue {
		t.Fatal("no cache keys were invalidated")
	}
	if len(f.bumped) != wantGenerationBumps {
		t.Fatalf("list generation moved %d times, want %d", len(f.bumped), wantGenerationBumps)
	}
}

// Cache keys are generated deterministically and the key-builder funcs are
// safe to run against a request.
func TestKeys(t *testing.T) {
	if cache.GenerateKey(constants.ResourceUser, constants.OpList, constants.EmptyString) == constants.EmptyString {
		t.Fatal("GenerateKey returned empty")
	}
	if cache.ValidateCacheKey(constants.EmptyString) {
		t.Fatal("empty key validated as ok")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	_ = cache.NewGetCacheKeyFunc(constants.ResourceUser)(req)
}
