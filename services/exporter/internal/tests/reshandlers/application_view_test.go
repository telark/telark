package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	applicationexp "github.com/telark/exporter/internal/exporters/application"
	"github.com/telark/exporter/internal/utils/performance"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/shared"
	"github.com/telark/rest/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applicationList() *unstructured.UnstructuredList {
	return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]any{
		"spec": map[string]any{
			"name":      "app-1",
			"resources": []any{map[string]any{"kind": "Deployment"}},
			"snapshots": []any{map[string]any{"path": "/snapshots/apps/id/ns/V1.json"}},
			"rollbacks": []any{map[string]any{"id": "rb-1"}},
			"metrics":   map[string]any{"score": 1, "workloads": []any{map[string]any{"name": "w"}}},
			"history": map[string]any{
				"generation": 2,
				"changeLog":  []any{map[string]any{"generation": 2, "changes": []any{map[string]any{"field": "x"}}}},
			},
		},
	}}}}
}

// The summary view is served under its own cache key: the pruned blob must never
// reach a full-view caller such as discovery's history diff.
func TestApplicationSummaryViewIsCachedApartFromTheFullList(t *testing.T) {
	o := newOptimizer(t)
	inner := func(w http.ResponseWriter, r *http.Request) {
		result := shared.CreateKubernetesAPIData(http.StatusOK, "", applicationList(), nil)
		if r.URL.Query().Get(constants.ViewParam) == constants.ViewSummary {
			applicationexp.SendApplicationSummaries(w, result)
			return
		}
		full, _ := sharedutils.FilterData(result.Data)
		sharedutils.LogByStatusAndSend(w, http.StatusOK, response.OperationSuccess, "", full, nil)
	}
	keyFunc := cache.NewViewListCacheKeyFunc(o, constants.ResourceApplication)
	handler := performance.NewCachedListHandlerFunc(o, inner, keyFunc, constants.ResourceApplication, constants.OpList)
	serve := func(target string) string {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Body.String()
	}
	pruned := []string{`"resources"`, `"snapshots"`, `"rollbacks"`, `"workloads"`, `"changes"`}
	kept := []string{`"name":"app-1"`, `"score"`, `"generation":2`, `"changeLog"`}

	summary := serve("/applications?view=summary")
	for _, key := range pruned {
		if strings.Contains(summary, key) {
			t.Errorf("summary still carries %s: %s", key, summary)
		}
	}
	for _, key := range kept {
		if !strings.Contains(summary, key) {
			t.Errorf("summary lost %s: %s", key, summary)
		}
	}

	full := serve("/applications")
	for _, key := range append(pruned, kept...) {
		if !strings.Contains(full, key) {
			t.Errorf("full view lost %s: %s", key, full)
		}
	}

	summaryKey := keyFunc(httptest.NewRequest(http.MethodGet, "/applications?view=summary", nil))
	fullKey := keyFunc(httptest.NewRequest(http.MethodGet, "/applications", nil))
	if summaryKey == fullKey {
		t.Fatalf("summary and full views share cache key %q", summaryKey)
	}
}
