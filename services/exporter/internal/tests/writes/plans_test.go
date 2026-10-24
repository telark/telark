package writes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	plansPath = "/api/v1/protectionplans"
	newPlanID = "pp-abc-1234-5678"
	newPlan   = `{"id":"pp-abc-1234-5678","name":"freeze","policies":[{"templateID":"block-create"}],` +
		`"scope":{"type":"applications","applicationRefs":["app-1"]}}`
	badRepeatPatch = `{"repeat":{"days":["sun"],"startTime":"02:00","endTime":"04:00","timeZone":"Mars/Olympus"}}`
)

// Writes from another service name the acting user in a header.
func serviceCall(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set(constants.HeaderUserID, userID)
	r = r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{Internal: true}))
	w := httptest.NewRecorder()
	router.NewRouter(routes.InitRoutes(optimizer, nil)).ServeHTTP(w, r)
	return w
}

// A created plan is stored; a patch carrying a rule in an unknown zone is a 400 and never stored.
func TestPlanCreateAndARefusedRepeatPatch(t *testing.T) {
	client := installFake(t)
	if w := serviceCall(t, http.MethodPost, plansPath, newPlan); w.Code != http.StatusOK {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	md := v1alpha1.ProtectionPlanMetadata
	if _, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).Get(t.Context(), newPlanID, metav1.GetOptions{}); err != nil {
		t.Fatalf("stored plan: %v", err)
	}
	client.ClearActions()
	if w := serviceCall(t, http.MethodPatch, plansPath+"/"+newPlanID, badRepeatPatch); w.Code != http.StatusBadRequest {
		t.Fatalf("patch = %d %s, want 400", w.Code, w.Body.String())
	}
	if got := writes(t, client); len(got) != constants.DefaultInitValue {
		t.Fatalf("writes after the refused patch = %+v, want none", got)
	}
}
