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
	sharedutils "github.com/telark/exporter/internal/utils/shared"
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

	del := httptest.NewRecorder()
	exportshared.DeleteResource(del, md, testResourceName)
	expectError(t, del, "DeleteResource")

	patch := httptest.NewRecorder()
	exportshared.PatchResource(patch, namedReq(`{"name":"n1"}`), md)
	expectError(t, patch, "PatchResource")
}

// The parse error used to reach the caller as "…request body: %v: <cause>".
func TestMalformedBodyMessageIsFormatted(t *testing.T) {
	md := metadata.ApplicationAsResourceMetadata
	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"PatchResource", func(w http.ResponseWriter, r *http.Request) { exportshared.PatchResource(w, r, md) }},
		{"GetSpec", func(w http.ResponseWriter, r *http.Request) { _, _ = sharedutils.GetSpec(w, r) }},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		tt.call(rec, namedReq(`{not json`))
		body := rec.Body.String()
		if rec.Code != http.StatusUnprocessableEntity || strings.Contains(body, "%v") || !strings.Contains(body, "invalid character") {
			t.Errorf("%s: code = %d body = %s, want 422 with the formatted cause", tt.name, rec.Code, body)
		}
	}
}

// An empty name is a client error, rejected before the API server is reached.
func TestGenericEmptyNameIsBadRequest(t *testing.T) {
	md := metadata.RoleAsResourceMetadata

	get := httptest.NewRecorder()
	generics.GenericGetCustomResource(get, constants.EmptyString, md)
	patch := httptest.NewRecorder()
	generics.GenericPatchCustomResource(patch, md, constants.EmptyString, map[string]any{constants.SpecField: map[string]any{}})

	for name, rec := range map[string]*httptest.ResponseRecorder{"GenericGet": get, "GenericPatch": patch} {
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want %d", name, rec.Code, http.StatusBadRequest)
		}
	}
}
