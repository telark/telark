package analyzefailclosed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/handlers/analyze/resources"
	"github.com/telark/telark/services/discovery/internal/handlers/analyze/workloads"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

func list(ctx context.Context, handler http.HandlerFunc) int {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, mux.SetURLVars(req, map[string]string{constants.NamespaceParam: "kube-system"}))
	return rec.Code
}

// Alone in its package, because the first excluded list loaded stays for the process: until a
// list was ever loaded, every namespace is refused instead of listing Secret names from an excluded one.
func TestAnalyzeListsFailClosedWithoutExcludedList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	testutil.Equal(t, "resources", list(ctx, resources.ListNamespaceResources), http.StatusServiceUnavailable)
	testutil.Equal(t, "workloads", list(ctx, workloads.ListNamespaceWorkloads), http.StatusServiceUnavailable)
}
