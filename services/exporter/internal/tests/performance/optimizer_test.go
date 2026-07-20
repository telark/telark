package performance

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
)

func newOptimizer(t *testing.T) *performance.Optimizer {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	o, err := performance.NewOptimizer()
	if err != nil {
		t.Fatalf("NewOptimizer: %v", err)
	}
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

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"data":{"x":1}}`))
}

func TestCachedListHandlerMissThenHit(t *testing.T) {
	o := newOptimizer(t)
	handler := performance.NewCachedListHandlerFunc(
		o, okHandler, performance.KeyFunc, constants.ResourceApplication, constants.OpList,
	)

	req := httptest.NewRequest(http.MethodGet, "/applications?x=1", nil)
	miss := httptest.NewRecorder()
	handler(miss, req)
	if miss.Code != http.StatusOK {
		t.Fatalf("miss code = %d, want 200", miss.Code)
	}

	hit := httptest.NewRecorder()
	handler(hit, httptest.NewRequest(http.MethodGet, "/applications?x=1", nil))
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
		o, failing, performance.KeyFunc, constants.ResourceApplication, constants.OpList,
	)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/applications", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}

func TestOptimizedHandler(t *testing.T) {
	o := newOptimizer(t)
	handler := performance.NewDynamicOptimizedHandlerFunc(
		o, okHandler, constants.ResourceApplication, constants.OpGet,
	)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/applications/a1", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("OptimizedHandler code = %d, want 200", rec.Code)
	}
}
