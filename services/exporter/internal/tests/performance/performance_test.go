package performance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func TestGetTimeoutForResource(t *testing.T) {
	// A resource+operation configured explicitly returns its specific timeout.
	if got := performance.GetTimeoutForResource(constants.ResourceApplication, constants.OpGet); got <= 0 {
		t.Errorf("configured timeout = %v, want > 0", got)
	}
	// An unconfigured resource falls back to the per-operation default.
	if got := performance.GetTimeoutForResource("unconfigured", constants.OpCreate); got <= 0 {
		t.Errorf("fallback timeout = %v, want > 0", got)
	}
	// An unknown operation lands on the global default.
	if got := performance.GetTimeoutForResource("unconfigured", "unknown-op"); got <= 0 {
		t.Errorf("default timeout = %v, want > 0", got)
	}
}

func TestKeyFunc(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/resources?limit=10", nil)
	if got := performance.KeyFunc(r); got != "cache:/resources?limit=10" {
		t.Errorf("KeyFunc = %q", got)
	}
}

func serveList(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func listBody(w http.ResponseWriter, r *http.Request) {
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, "listed", map[string]any{"path": r.URL.Path}, nil)
}

// Two renders may run at once by default; a third, on its own key, waits the
// render window and is refused with Retry-After instead of piling up.
func TestCachedListHandlerBoundsConcurrentRenders(t *testing.T) {
	o := newOptimizer(t)
	started := make(chan struct{}, constants.DefaultListRenderConcurrency)
	release := make(chan struct{})
	blocking := func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-release
		listBody(w, r)
	}
	keyByPath := func(r *http.Request) string { return "list:test" + r.URL.Path }
	handler := performance.NewCachedListHandlerFunc(o, blocking, keyByPath, constants.ResourceApplication, constants.OpList)

	var wg sync.WaitGroup
	codes := make([]int, constants.DefaultListRenderConcurrency)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = serveList(handler, "/"+strconv.Itoa(i)).Code
		}()
	}
	for range codes {
		<-started
	}

	refused := serveList(handler, "/beyond")
	if refused.Code != http.StatusServiceUnavailable || refused.Header().Get(constants.HeaderRetryAfter) != constants.ListRenderRetryAfter {
		t.Fatalf("beyond capacity: code = %d, Retry-After = %q", refused.Code, refused.Header().Get(constants.HeaderRetryAfter))
	}
	if refused.Header().Get(constants.HeaderETag) != constants.EmptyString {
		t.Error("refused response carries a validator")
	}
	close(release)
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("render %d: code = %d, want 200", i, code)
		}
	}
}

// A repeat under the current key is served from the in-process blob: the Redis
// copy is deleted between the requests and the handler still renders once. The
// hit is spliced into the same envelope the live response used.
func TestCachedListHandlerServesLocalBlobWithoutRedis(t *testing.T) {
	o := newOptimizer(t)
	renders := 0
	live := func(w http.ResponseWriter, r *http.Request) {
		renders++
		listBody(w, r)
	}
	keyFunc := cache.NewListCacheKeyFunc(o, constants.ResourceApplication)
	handler := performance.NewCachedListHandlerFunc(o, live, keyFunc, constants.ResourceApplication, constants.OpList)

	first := serveList(handler, "/applications")
	o.Delete(keyFunc(httptest.NewRequest(http.MethodGet, "/applications", nil)))
	second := serveList(handler, "/applications")
	if renders != 1 || second.Code != http.StatusOK {
		t.Fatalf("renders = %d, second code = %d, want one render and a local hit", renders, second.Code)
	}

	var liveResp, hitResp response.GenericResponse
	if err := json.Unmarshal(first.Body.Bytes(), &liveResp); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Body.Bytes(), &hitResp); err != nil {
		t.Fatalf("cached body is not a response envelope: %v: %s", err, second.Body.String())
	}
	if hitResp.Status != liveResp.Status || hitResp.Operation != liveResp.Operation || !reflect.DeepEqual(hitResp.Data, liveResp.Data) {
		t.Errorf("hit = %+v, live = %+v", hitResp, liveResp)
	}
	if hitResp.Message != constants.CachedResponse {
		t.Errorf("hit message = %q, want %q", hitResp.Message, constants.CachedResponse)
	}
	if got := second.Header().Get(constants.HeaderContentLength); got != strconv.Itoa(second.Body.Len()) {
		t.Errorf("Content-Length = %q, body = %d bytes", got, second.Body.Len())
	}
	if got := second.Header().Get(constants.HeaderContentType); got != constants.ContentTypeJSON {
		t.Errorf("Content-Type = %q, want %q", got, constants.ContentTypeJSON)
	}
}
