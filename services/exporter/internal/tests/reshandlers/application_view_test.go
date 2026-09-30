package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	applicationexp "github.com/telark/exporter/internal/exporters/application"
	apphandler "github.com/telark/exporter/internal/handlers/resources/application"
	"github.com/telark/exporter/internal/informers"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/kcore/shared"
	"github.com/telark/rest/base"
	restconstants "github.com/telark/rest/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8scache "k8s.io/client-go/tools/cache"
)

const (
	testAppName      = "app-1"
	applicationsPath = "/applications"
	summaryViewPath  = applicationsPath + "?view=summary"

	testScore             = 1
	testHistoryGeneration = 2
)

var (
	summaryPruned = []string{`"resources"`, `"snapshots"`, `"rollbacks"`, `"workloads"`, `"changes"`}
	summaryKept   = []string{`"name":"app-1"`, `"score"`, `"generation":2`, `"changeLog"`}
)

// Observed state lives in .status; the list view projects it flat next to the spec.
func applicationStatus() map[string]any {
	return map[string]any{
		"resources": []any{map[string]any{"kind": "Deployment"}},
		"snapshots": []any{map[string]any{"path": "/snapshots/apps/id/ns/V1.json"}},
		"rollbacks": []any{map[string]any{"id": "rb-1"}},
		"metrics":   map[string]any{"score": testScore, "workloads": []any{map[string]any{constants.NameParam: "w"}}},
		"history": map[string]any{
			"generation": testHistoryGeneration,
			"changeLog":  []any{map[string]any{"generation": testHistoryGeneration, "changes": []any{map[string]any{"field": "x"}}}},
		},
	}
}

func applicationList() *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{*storedApplication(testAppName)}}
}

func applicationViewInner(w http.ResponseWriter, r *http.Request) {
	result := shared.CreateKubernetesAPIData(http.StatusOK, "", applicationList(), nil)
	applicationexp.SendApplicationList(w, result, r.URL.Query().Get(constants.ViewParam))
}

func storedApplication(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":        "telark.io/v1alpha1",
		"kind":              "Application",
		"metadata":          map[string]any{constants.NameParam: name, "namespace": "telark"},
		constants.SpecField: map[string]any{constants.NameParam: name},
		"status":            applicationStatus(),
	}}
}

func useStore(t *testing.T, synced bool, apps ...*unstructured.Unstructured) {
	t.Helper()
	store := k8scache.NewStore(k8scache.MetaNamespaceKeyFunc)
	for _, app := range apps {
		if err := store.Add(app); err != nil {
			t.Fatal(err)
		}
	}
	informers.Use(store, func() bool { return synced })
	t.Cleanup(func() { informers.Use(nil, nil) })
}

func assertKeys(t *testing.T, view string, body string, keys []string, want bool) {
	t.Helper()
	for _, key := range keys {
		if strings.Contains(body, key) != want {
			t.Errorf("%s view: %s present = %v, want %v: %s", view, key, !want, want, body)
		}
	}
}

// Nothing here can reach an apiserver, so a 200 proves both views were rendered
// from the store; the stored object must come out of the summary pruning
// untouched, and a fresh read must still go to the apiserver.
func TestApplicationListServedFromInformerStore(t *testing.T) {
	app := storedApplication(testAppName)
	useStore(t, true, app)
	handler := apphandler.ListApplicationResourcesWithCacheInvalidation()

	summary := serveView(handler, summaryViewPath, constants.EmptyString)
	if summary.Code != http.StatusOK {
		t.Fatalf("summary from store: code = %d, body = %s", summary.Code, summary.Body.String())
	}
	assertKeys(t, constants.ViewSummary, summary.Body.String(), summaryPruned, false)
	assertKeys(t, constants.ViewSummary, summary.Body.String(), summaryKept, true)

	full := serveView(handler, applicationsPath, constants.EmptyString)
	if full.Code != http.StatusOK {
		t.Fatalf("full from store: code = %d, body = %s", full.Code, full.Body.String())
	}
	assertKeys(t, constants.ViewFull, full.Body.String(), append(summaryPruned, summaryKept...), true)

	status, ok := app.Object["status"].(map[string]any)
	if !ok {
		t.Fatal("stored application lost its status map")
	}
	if _, kept := status["resources"]; !kept {
		t.Error("summary pruning mutated the stored status")
	}

	rec := httptest.NewRecorder()
	fresh := httptest.NewRequest(http.MethodGet, applicationsPath, nil)
	fresh.Header.Set(restconstants.HeaderCacheControl, restconstants.CacheControlNoCache)
	handler(rec, fresh)
	if rec.Code == http.StatusOK {
		t.Errorf("fresh read: code = %d, want the apiserver path rather than the store", rec.Code)
	}
}

func TestApplicationListFallsBackBeforeInformerSync(t *testing.T) {
	useStore(t, false, storedApplication(testAppName))
	rec := serveView(apphandler.ListApplicationResourcesWithCacheInvalidation(), applicationsPath, constants.EmptyString)
	if rec.Code == http.StatusOK {
		t.Errorf("unsynced store: code = %d, want the apiserver path", rec.Code)
	}
}

func serveView(handler http.HandlerFunc, target string, ifNoneMatch string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if ifNoneMatch != constants.EmptyString {
		req.Header.Set(constants.HeaderIfNoneMatch, ifNoneMatch)
	}
	handler(rec, req)
	return rec
}

// The summary view is served under its own cache key: the pruned blob must never
// reach a full-view caller such as discovery's history diff.
func TestApplicationSummaryViewIsCachedApartFromTheFullList(t *testing.T) {
	o := newOptimizer(t)
	keyFunc := cache.NewViewListCacheKeyFunc(o, constants.ResourceApplication)
	handler := performance.NewCachedListHandlerFunc(o, applicationViewInner, keyFunc, constants.ResourceApplication, constants.OpList)

	summary := serveView(handler, summaryViewPath, constants.EmptyString).Body.String()
	assertKeys(t, constants.ViewSummary, summary, summaryPruned, false)
	assertKeys(t, constants.ViewSummary, summary, summaryKept, true)

	full := serveView(handler, applicationsPath, constants.EmptyString).Body.String()
	assertKeys(t, constants.ViewFull, full, append(summaryPruned, summaryKept...), true)

	summaryKey := keyFunc(httptest.NewRequest(http.MethodGet, summaryViewPath, nil))
	fullKey := keyFunc(httptest.NewRequest(http.MethodGet, applicationsPath, nil))
	if summaryKey == fullKey {
		t.Fatalf("summary and full views share cache key %q", summaryKey)
	}
}

// A poller holding the current generation's validator is answered 304 before
// the blob is read; a bump or another view makes that validator miss.
func TestApplicationListRevalidatesByGeneration(t *testing.T) {
	o := newOptimizer(t)
	keyFunc := cache.NewViewListCacheKeyFunc(o, constants.ResourceApplication)
	handler := performance.NewCachedListHandlerFunc(o, applicationViewInner, keyFunc, constants.ResourceApplication, constants.OpList)
	summary := summaryViewPath

	first := serveView(handler, summary, constants.EmptyString)
	etag := first.Header().Get(constants.HeaderETag)
	if first.Code != http.StatusOK || !strings.HasPrefix(etag, constants.WeakETagPrefix) {
		t.Fatalf("first GET: code = %d, ETag = %q", first.Code, etag)
	}
	if got := first.Header().Get(restconstants.HeaderCacheControl); got != restconstants.CacheControlNoCache {
		t.Errorf("Cache-Control = %q, want %q", got, restconstants.CacheControlNoCache)
	}

	again := serveView(handler, summary, etag)
	if again.Code != http.StatusNotModified || again.Body.Len() != constants.DefaultInitValue || again.Header().Get(constants.HeaderETag) != etag {
		t.Fatalf("revalidation: code = %d, body = %d bytes, ETag = %q", again.Code, again.Body.Len(), again.Header().Get(constants.HeaderETag))
	}

	full := serveView(handler, applicationsPath, etag)
	if full.Code != http.StatusOK || full.Header().Get(constants.HeaderETag) == etag {
		t.Errorf("full view: code = %d, ETag = %q, want 200 under its own validator", full.Code, full.Header().Get(constants.HeaderETag))
	}

	o.BumpListGeneration(constants.ResourceApplication)
	bumped := serveView(handler, summary, etag)
	if bumped.Code != http.StatusOK || bumped.Header().Get(constants.HeaderETag) == etag {
		t.Errorf("after bump: code = %d, ETag = %q, want 200 under a new validator", bumped.Code, bumped.Header().Get(constants.HeaderETag))
	}
}

// A single-resource key has no generation, and an error body must never be
// revalidated into a 304: neither response carries a validator.
func TestValidatorOnlyOnListSuccess(t *testing.T) {
	failing := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, r.URL.Path, http.StatusServiceUnavailable)
	}
	cases := []struct {
		name      string
		handler   http.HandlerFunc
		operation string
		code      int
	}{
		{"get", applicationViewInner, constants.OpGet, http.StatusOK},
		{"error", failing, constants.OpList, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		o := newOptimizer(t)
		keyFunc := cache.NewViewListCacheKeyFunc(o, constants.ResourceApplication)
		handler := performance.NewCachedListHandlerFunc(o, tc.handler, keyFunc, constants.ResourceApplication, tc.operation)
		rec := serveView(handler, applicationsPath, constants.EmptyString)
		if rec.Code != tc.code || rec.Header().Get(constants.HeaderETag) != constants.EmptyString {
			t.Errorf("%s: code = %d, ETag = %q, want %d without a validator", tc.name, rec.Code, rec.Header().Get(constants.HeaderETag), tc.code)
		}
	}
}

// The editable-field guard reads the body first; an oversized one must still reach the downstream parser whole.
func TestApplicationPatchOverLimitIsTooLarge(t *testing.T) {
	body := `{"description":"` + strings.Repeat("x", int(base.MaxRequestBodySize)) + `"}`
	r := httptest.NewRequest(http.MethodPatch, applicationsPath, strings.NewReader(body))
	r = mux.SetURLVars(r, map[string]string{constants.NameParam: testAppName})
	rec := httptest.NewRecorder()
	apphandler.PatchApplicationResourceWithCacheInvalidation(newOptimizer(t))(rec, r)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}
