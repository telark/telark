package analyzehandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/handlers/analyze/resources"
	"github.com/telark/discovery/internal/handlers/analyze/workloads"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	"github.com/telark/discovery/internal/tests/testutil"
)

const kubeSystem = "kube-system"

func list(ctx context.Context, handler http.HandlerFunc) int {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, mux.SetURLVars(req, map[string]string{constants.NamespaceParam: kubeSystem}))
	return rec.Code
}

// Declared before the test that loads the list: until a list was ever loaded, every namespace
// is refused instead of listing Secret names from an excluded one.
func TestAnalyzeListsFailClosedWithoutExcludedList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	testutil.Equal(t, "resources", list(ctx, resources.ListNamespaceResources), http.StatusServiceUnavailable)
	testutil.Equal(t, "workloads", list(ctx, workloads.ListNamespaceWorkloads), http.StatusServiceUnavailable)
}

func TestAnalyzeListsRefuseExcludedNamespaces(t *testing.T) {
	gcfghelper.SetExcludedForTest([]string{kubeSystem})
	ctx := context.Background()
	testutil.Equal(t, "resources", list(ctx, resources.ListNamespaceResources), http.StatusForbidden)
	testutil.Equal(t, "workloads", list(ctx, workloads.ListNamespaceWorkloads), http.StatusForbidden)
}
