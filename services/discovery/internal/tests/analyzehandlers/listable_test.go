package analyzehandlers

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

const kubeSystem = "kube-system"

func list(ctx context.Context, handler http.HandlerFunc) int {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler(rec, mux.SetURLVars(req, map[string]string{constants.NamespaceParam: kubeSystem}))
	return rec.Code
}

func TestAnalyzeListsRefuseExcludedNamespaces(t *testing.T) {
	testutil.ExcludedNamespaces(t, kubeSystem)
	ctx := context.Background()
	testutil.Equal(t, "resources", list(ctx, resources.ListNamespaceResources), http.StatusForbidden)
	testutil.Equal(t, "workloads", list(ctx, workloads.ListNamespaceWorkloads), http.StatusForbidden)
}
