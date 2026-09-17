package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/constants"
	categoryhandler "github.com/telark/exporter/internal/handlers/classification/category"
	protectionhandler "github.com/telark/exporter/internal/handlers/plans/protection"
	grouphandler "github.com/telark/exporter/internal/handlers/resources/group"
	rolehandler "github.com/telark/exporter/internal/handlers/resources/role"
	userhandler "github.com/telark/exporter/internal/handlers/resources/user"
	"github.com/telark/exporter/internal/utils/performance"
)

func newOptimizer(t *testing.T) *performance.Optimizer {
	t.Helper()
	mr := miniredis.RunT(t)
	o := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	t.Cleanup(o.Close)
	return o
}

func idReq(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/resource/"+id, nil)
	return mux.SetURLVars(r, map[string]string{constants.IDParam: id})
}

func postJSON(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(body))
}

// Each handler, driven without a reachable cluster, must answer with an error
// status rather than a success or a panic.
func assertErrorResponse(t *testing.T, h http.HandlerFunc, r *http.Request, name string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, r)
	if rec.Code < http.StatusBadRequest {
		t.Errorf("%s: code = %d, want an error status", name, rec.Code)
	}
}

func TestRoleHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, rolehandler.CreateRoleResourceWithCacheInvalidation(o), postJSON("{}"), "CreateRole")
	assertErrorResponse(t, rolehandler.GetRoleByIDWithCacheInvalidation(), idReq("r1"), "GetRoleByID")
	assertErrorResponse(t, rolehandler.ListRoleResourcesWithCacheInvalidation(o), idReq("r1"), "ListRoles")
	assertErrorResponse(t, rolehandler.PatchRoleByIDWithCacheInvalidation(o), idReq("r1"), "PatchRole")
	assertErrorResponse(t, rolehandler.DeleteRoleByIDWithCacheInvalidation(o), idReq("r1"), "DeleteRole")
}

func TestUserHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, userhandler.CreateUserResourceWithCacheInvalidation(o), postJSON("{}"), "CreateUser")
	assertErrorResponse(t, userhandler.GetUserByIDWithCacheInvalidation(), idReq("u1"), "GetUserByID")
	assertErrorResponse(t, userhandler.ListUserResourcesWithCacheInvalidation(o), idReq("u1"), "ListUsers")
	assertErrorResponse(t, userhandler.PatchUserByIDWithCacheInvalidation(o), idReq("u1"), "PatchUser")
	assertErrorResponse(t, userhandler.DeleteUserByIDWithCacheInvalidation(o), idReq("u1"), "DeleteUser")
}

func TestGroupHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, grouphandler.CreateGroupResourceWithCacheInvalidation(o), postJSON("{}"), "CreateGroup")
	assertErrorResponse(t, grouphandler.GetGroupByIDWithCacheInvalidation(), idReq("g1"), "GetGroupByID")
	assertErrorResponse(t, grouphandler.ListGroupResourcesWithCacheInvalidation(o), idReq("g1"), "ListGroups")
	assertErrorResponse(t, grouphandler.PatchGroupByIDWithCacheInvalidation(o), idReq("g1"), "PatchGroup")
	assertErrorResponse(t, grouphandler.DeleteGroupByIDWithCacheInvalidation(o), idReq("g1"), "DeleteGroup")
}

func TestCategoryHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, categoryhandler.CreateCategoryResourceWithCacheInvalidation(o), postJSON("{}"), "CreateCategory")
	assertErrorResponse(t, categoryhandler.GetCategoryByIDWithCacheInvalidation(), idReq("c1"), "GetCategoryByID")
	assertErrorResponse(t, categoryhandler.ListAllCategoriesWithCacheInvalidation(), idReq("c1"), "ListCategories")
	assertErrorResponse(t, categoryhandler.PatchCategoryByIDWithCacheInvalidation(o), idReq("c1"), "PatchCategory")
	assertErrorResponse(t, categoryhandler.DeleteCategoryByIDWithCacheInvalidation(o), idReq("c1"), "DeleteCategory")
}

func TestProtectionPlanHandlers(t *testing.T) {
	assertErrorResponse(t, protectionhandler.CreatePlan(), varsReq(http.MethodPost, "{}"), "CreatePlan")
	assertErrorResponse(t, protectionhandler.GetPlanByID(), idReq("p1"), "GetPlanByID")
	assertErrorResponse(t, protectionhandler.ListPlans(), idReq("p1"), "ListPlans")
	assertErrorResponse(t, protectionhandler.PatchPlanByID(), varsReq(http.MethodPatch, "{}"), "PatchPlanByID")
	assertErrorResponse(t, protectionhandler.DeletePlanByID(), idReq("p1"), "DeletePlanByID")
}
