package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/exporter/internal/constants"
	categoryhandler "github.com/telark/telark/services/exporter/internal/handlers/categories"
	protectionhandler "github.com/telark/telark/services/exporter/internal/handlers/plans/protection"
	grouphandler "github.com/telark/telark/services/exporter/internal/handlers/resources/group"
	rolehandler "github.com/telark/telark/services/exporter/internal/handlers/resources/role"
	userhandler "github.com/telark/telark/services/exporter/internal/handlers/resources/user"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

const (
	testRoleID     = "r1"
	testUserID     = "u1"
	testGroupID    = "g1"
	testCategoryID = "c1"
	testPlanID     = "p1"

	emptyJSONBody = "{}"
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

func postJSON() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(emptyJSONBody))
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
	assertErrorResponse(t, rolehandler.CreateRoleResourceWithCacheInvalidation(o), postJSON(), "CreateRole")
	assertErrorResponse(t, rolehandler.GetRoleByIDWithCacheInvalidation(), idReq(testRoleID), "GetRoleByID")
	assertErrorResponse(t, rolehandler.ListRoleResourcesWithCacheInvalidation(), idReq(testRoleID), "ListRoles")
	assertErrorResponse(t, rolehandler.PatchRoleByIDWithCacheInvalidation(o), idReq(testRoleID), "PatchRole")
	assertErrorResponse(t, rolehandler.DeleteRoleByIDWithCacheInvalidation(o), idReq(testRoleID), "DeleteRole")
}

func TestUserHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, userhandler.CreateUserResourceWithCacheInvalidation(o), postJSON(), "CreateUser")
	assertErrorResponse(t, userhandler.GetUserByIDWithCacheInvalidation(), idReq(testUserID), "GetUserByID")
	assertErrorResponse(t, userhandler.ListUserResourcesWithCacheInvalidation(), idReq(testUserID), "ListUsers")
	assertErrorResponse(t, userhandler.PatchUserByIDWithCacheInvalidation(o), idReq(testUserID), "PatchUser")
	assertErrorResponse(t, userhandler.DeleteUserByIDWithCacheInvalidation(o), idReq(testUserID), "DeleteUser")
}

func TestGroupHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, grouphandler.CreateGroupResourceWithCacheInvalidation(o), postJSON(), "CreateGroup")
	assertErrorResponse(t, grouphandler.GetGroupByIDWithCacheInvalidation(), idReq(testGroupID), "GetGroupByID")
	assertErrorResponse(t, grouphandler.ListGroupResourcesWithCacheInvalidation(), idReq(testGroupID), "ListGroups")
	assertErrorResponse(t, grouphandler.PatchGroupByIDWithCacheInvalidation(o), idReq(testGroupID), "PatchGroup")
	assertErrorResponse(t, grouphandler.DeleteGroupByIDWithCacheInvalidation(o), idReq(testGroupID), "DeleteGroup")
}

func TestCategoryHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, categoryhandler.CreateCategoryResourceWithCacheInvalidation(o), postJSON(), "CreateCategory")
	assertErrorResponse(t, categoryhandler.GetCategoryByIDWithCacheInvalidation(), idReq(testCategoryID), "GetCategoryByID")
	assertErrorResponse(t, categoryhandler.ListCategoriesWithCacheInvalidation(), idReq(testCategoryID), "ListCategories")
	assertErrorResponse(t, categoryhandler.PatchCategoryByIDWithCacheInvalidation(o), idReq(testCategoryID), "PatchCategory")
	assertErrorResponse(t, categoryhandler.DeleteCategoryByIDWithCacheInvalidation(o), idReq(testCategoryID), "DeleteCategory")
}

func TestProtectionPlanHandlers(t *testing.T) {
	assertErrorResponse(t, protectionhandler.CreatePlan(), varsReq(http.MethodPost, emptyJSONBody), "CreatePlan")
	assertErrorResponse(t, protectionhandler.GetPlanByID(), idReq(testPlanID), "GetPlanByID")
	assertErrorResponse(t, protectionhandler.ListPlans(), idReq(testPlanID), "ListPlans")
	assertErrorResponse(t, protectionhandler.PatchPlanByID(), varsReq(http.MethodPatch, emptyJSONBody), "PatchPlanByID")
	assertErrorResponse(t, protectionhandler.DeletePlanByID(), idReq(testPlanID), "DeletePlanByID")
}
