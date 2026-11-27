package performance

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

func newOptimizer(t *testing.T) *performance.Optimizer {
	t.Helper()
	mr := miniredis.RunT(t)
	o := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(o.Close)
	return o
}

func TestOptimizerRoundTrip(t *testing.T) {
	o := newOptimizer(t)
	key := "cache:/resource?x=1"
	o.Set(key, "payload")
	if _, ok := o.Get(key); !ok {
		t.Error("value not retrievable after Set")
	}
	o.Delete(key)
	if _, ok := o.Get(key); ok {
		t.Error("value still present after Delete")
	}
	o.Clear()
}

// A bump deferred by the write window outlives every list it retires: its flag once expired
// after a minute, and the next reader kept the old generation and a list up to CacheTTL old.
func TestDeferredBumpOutlivesTheCachedList(t *testing.T) {
	mr := miniredis.RunT(t)
	o := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(o.Close)
	o.BumpListGeneration(constants.ResourceRole)
	before := o.ListGeneration(constants.ResourceRole)
	o.BumpListGeneration(constants.ResourceRole)

	mr.FastForward(constants.CacheTTL - time.Second)

	if got := o.ListGeneration(constants.ResourceRole); got == before {
		t.Fatalf("generation = %q after the window, want the deferred bump applied", got)
	}
}

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"data":{"x":1}}`))
}

func keyFunc(r *http.Request) string {
	return "cache:" + r.URL.Path + "?" + r.URL.RawQuery
}

func TestCachedListHandlerMissThenHit(t *testing.T) {
	o := newOptimizer(t)
	handler := performance.NewCachedListHandlerFunc(
		o, okHandler, keyFunc, constants.ResourceApplication, constants.OpList,
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications?x=1", nil)
	miss := httptest.NewRecorder()
	handler(miss, req)
	if miss.Code != http.StatusOK {
		t.Fatalf("miss code = %d, want 200", miss.Code)
	}

	hit := httptest.NewRecorder()
	handler(hit, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications?x=1", nil))
	if hit.Code != http.StatusOK {
		t.Errorf("hit code = %d, want 200", hit.Code)
	}
}

func TestCachedListHandlerSkipsNon200(t *testing.T) {
	o := newOptimizer(t)
	failing := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}
	handler := performance.NewCachedListHandlerFunc(
		o, failing, keyFunc, constants.ResourceApplication, constants.OpList,
	)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestOptimizedHandler(t *testing.T) {
	handler := performance.NewDynamicOptimizedHandlerFunc(
		okHandler, constants.ResourceApplication, constants.OpGet,
	)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/applications/a1", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("OptimizedHandler code = %d, want 200", rec.Code)
	}
}
