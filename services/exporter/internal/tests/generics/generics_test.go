package generics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	exportshared "github.com/telark/exporter/internal/exporters/shared"
)

const testResourceName = "n1"

func namedReq(body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(http.MethodGet, "/resource/n1", nil)
	} else {
		r = httptest.NewRequest(http.MethodPost, "/resource", strings.NewReader(body))
	}
	return mux.SetURLVars(r, map[string]string{constants.NameParam: testResourceName, constants.IDParam: testResourceName})
}

func expectError(t *testing.T, w *httptest.ResponseRecorder, name string) {
	t.Helper()
	if w.Code < http.StatusBadRequest {
		t.Errorf("%s: code = %d, want an error status", name, w.Code)
	}
}

// With no cluster reachable, every generic CRD operation must surface an error
// response instead of a success.
func TestGenericCRDOperations(t *testing.T) {
	md := metadata.RoleAsResourceMetadata
	spec := map[string]any{constants.SpecField: map[string]any{"name": testResourceName}}

	rec := httptest.NewRecorder()
	generics.GenericGetCustomResource(rec, testResourceName, md)
	expectError(t, rec, "GenericGet")

	list := httptest.NewRecorder()
	generics.GenericListCustomResources(list, md)
	expectError(t, list, "GenericList")

	create := httptest.NewRecorder()
	generics.GenericCreateCustomResource(create, md, testResourceName, map[string]any{"name": testResourceName})
	expectError(t, create, "GenericCreate")

	createFin := httptest.NewRecorder()
	generics.GenericCreateCustomResourceWithFinalizers(createFin, md, testResourceName, map[string]any{"name": testResourceName}, []string{"f"})
	expectError(t, createFin, "GenericCreateWithFinalizers")

	patch := httptest.NewRecorder()
	generics.GenericPatchCustomResource(patch, md, testResourceName, spec)
	expectError(t, patch, "GenericPatch")

	del := httptest.NewRecorder()
	generics.GenericDeleteCustomResource(del, md, testResourceName)
	expectError(t, del, "GenericDelete")
}

func TestSharedExporterOperations(t *testing.T) {
	md := metadata.RoleAsResourceMetadata

	create := httptest.NewRecorder()
	exportshared.CreateResource(create, md, testResourceName, map[string]any{"name": testResourceName})
	expectError(t, create, "CreateResource")

	getUnique := httptest.NewRecorder()
	exportshared.GetUniqueResourceFromList(getUnique, md)
	expectError(t, getUnique, "GetUniqueResourceFromList")

	del := httptest.NewRecorder()
	exportshared.DeleteResource(del, md, testResourceName)
	expectError(t, del, "DeleteResource")

	patch := httptest.NewRecorder()
	exportshared.PatchResource(patch, namedReq(`{"name":"n1"}`), md)
	expectError(t, patch, "PatchResource")
}
