package shared

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	dataerrors "github.com/telark/data/errors"
	basemeta "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidateRequiredField(t *testing.T) {
	if err := sharedutils.ValidateRequiredField("", "required"); err == nil {
		t.Error("empty value accepted")
	}
	if err := sharedutils.ValidateRequiredField("x", "required"); err != nil {
		t.Errorf("non-empty value rejected: %v", err)
	}
}

func TestGenerateResourceError(t *testing.T) {
	format := dataerrors.Error("resource %s failed: %v")
	withErr := sharedutils.GenerateResourceError(format, "app-1", http.ErrNoCookie)
	if !strings.Contains(withErr, "app-1") {
		t.Errorf("missing resource name: %q", withErr)
	}
	// A nil cause still yields a fully formatted message via the unknown-error fallback.
	if got := sharedutils.GenerateResourceError(format, "app-1", nil); !strings.Contains(got, "app-1") {
		t.Errorf("nil-error path lost the resource name: %q", got)
	}
}

func TestExtractResourceNameFromRequestBody(t *testing.T) {
	if got := sharedutils.ExtractResourceNameFromRequestBody(map[string]any{"name": "x"}); got != "x" {
		t.Errorf("got %q, want x", got)
	}
	if got := sharedutils.ExtractResourceNameFromRequestBody(map[string]any{}); got != "" {
		t.Errorf("missing name should be empty, got %q", got)
	}
}

func TestExtractMapValue(t *testing.T) {
	data := map[string]any{"m": map[string]any{"k": 1}, "s": "x"}
	if _, ok := sharedutils.ExtractMapValue(data, "m"); !ok {
		t.Error("map value not extracted")
	}
	if _, ok := sharedutils.ExtractMapValue(data, "s"); ok {
		t.Error("non-map value extracted as map")
	}
	if _, ok := sharedutils.ExtractMapValue(data, "absent"); ok {
		t.Error("absent key extracted")
	}
}

func TestRemoveAndAddBodyFields(t *testing.T) {
	body := map[string]any{"id": "x"}
	sharedutils.RemoveIDFromRequestBody(body)
	if _, ok := body["id"]; ok {
		t.Error("id not removed")
	}
	sharedutils.RemoveIDFromRequestBody(nil) // must not panic

	sharedutils.AddCreationDateToRequestBody(body)
	if _, ok := body[constants.FieldCreationDate]; !ok {
		t.Error("creation date not added")
	}
	body[constants.FieldCreationDate] = "keep"
	sharedutils.AddCreationDateToRequestBody(body)
	if body[constants.FieldCreationDate] != "keep" {
		t.Error("existing creation date overwritten")
	}
	sharedutils.AddCreationDateToRequestBody(nil) // must not panic
}

type sample struct {
	Name string `json:"name"`
}

func TestExtractStructFromBody(t *testing.T) {
	got, err := sharedutils.ExtractStructFromBody[sample](map[string]any{"name": "x"})
	if err != nil || got.Name != "x" {
		t.Fatalf("ExtractStructFromBody = %+v, err %v", got, err)
	}
	body := map[string]any{"id": "drop", "name": "y"}
	got, err = sharedutils.ExtractStructFromBodyIgnoringID[sample](body)
	if err != nil || got.Name != "y" {
		t.Fatalf("ExtractStructFromBodyIgnoringID = %+v, err %v", got, err)
	}
	if _, ok := body["id"]; ok {
		t.Error("id not stripped before extraction")
	}
}

func TestFilterDataSingleItem(t *testing.T) {
	item := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"field": "v"},
		"metadata": map[string]any{
			"resourceVersion": "12",
			"name":            "app-1",
			"extra":           "dropped",
		},
	}}
	out, err := sharedutils.FilterData(item)
	if err != nil {
		t.Fatalf("FilterData: %v", err)
	}
	res := out.(*unstructured.Unstructured)
	if res.Object["field"] != "v" {
		t.Error("spec field not surfaced")
	}
	meta := res.Object[constants.MetadataField].(map[string]any)
	if meta["resourceVersion"] != "12" || meta["name"] != "app-1" {
		t.Errorf("metadata subset wrong: %v", meta)
	}
	if _, ok := meta["extra"]; ok {
		t.Error("non-whitelisted metadata leaked")
	}
}

func TestFilterDataErrors(t *testing.T) {
	if _, err := sharedutils.FilterData("unsupported"); err == nil {
		t.Error("unsupported type accepted")
	}
	if _, err := sharedutils.FilterData(&unstructured.Unstructured{Object: map[string]any{}}); err == nil {
		t.Error("item without spec accepted")
	}
	if _, err := sharedutils.FilterData(&unstructured.Unstructured{Object: map[string]any{"spec": "notmap"}}); err == nil {
		t.Error("non-map spec accepted")
	}
}

func TestFilterDataList(t *testing.T) {
	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		{Object: map[string]any{"spec": map[string]any{"a": 1}}},
		{Object: map[string]any{"nospec": true}},
		{Object: map[string]any{"spec": "notmap"}},
	}}
	out, err := sharedutils.FilterData(list)
	if err != nil {
		t.Fatalf("FilterData list: %v", err)
	}
	filtered := out.(*unstructured.UnstructuredList)
	if len(filtered.Items) != 1 {
		t.Errorf("expected 1 valid item, got %d", len(filtered.Items))
	}
}

func TestFilterResourceOrRespond(t *testing.T) {
	valid := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"a": 1}}}
	if _, ok := sharedutils.FilterResourceOrRespond(valid); !ok {
		t.Error("valid resource rejected")
	}
	invalid := &unstructured.Unstructured{Object: map[string]any{}}
	if _, ok := sharedutils.FilterResourceOrRespond(invalid); ok {
		t.Error("invalid resource accepted")
	}
}

func TestConvertToCRDTemplate(t *testing.T) {
	md := basemeta.Metadata{BaseGroup: "erpi.telark", Kind: "Role", Version: "v1alpha1"}
	tmpl := sharedutils.ConvertToCRDTemplate(md, "app-1", map[string]any{"x": 1})
	if tmpl.Object["apiVersion"] != "erpi.telark/v1alpha1" || tmpl.Object["kind"] != "Role" {
		t.Errorf("bad envelope: %v", tmpl.Object)
	}
	withFin := sharedutils.ConvertToCRDTemplateWithFinalizers(md, "app-1", map[string]any{}, []string{"f1"})
	meta := withFin.Object["metadata"].(map[string]any)
	if meta["finalizers"] == nil {
		t.Error("finalizers not attached")
	}
	noFin := sharedutils.ConvertToCRDTemplateWithFinalizers(md, "app-1", map[string]any{}, nil)
	if _, ok := noFin.Object["metadata"].(map[string]any)["finalizers"]; ok {
		t.Error("empty finalizers should be omitted")
	}
	bare := sharedutils.ConvertToUnstructuredWithoutManagedFields(map[string]any{"a": 1})
	if bare.Object["spec"] == nil {
		t.Error("spec missing from bare unstructured")
	}
}

func TestStructAndUnstructuredConversion(t *testing.T) {
	spec, err := sharedutils.StructToSpecMap(sample{Name: "x"})
	if err != nil || spec["name"] != "x" {
		t.Fatalf("StructToSpecMap = %v, err %v", spec, err)
	}
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"name": "y"}}}
	got, err := sharedutils.UnstructuredToStruct[sample](res, "no spec", "bad spec", "unmarshal %v")
	if err != nil || got.Name != "y" {
		t.Fatalf("UnstructuredToStruct = %+v, err %v", got, err)
	}
	if _, err := sharedutils.UnstructuredToStruct[sample](&unstructured.Unstructured{Object: map[string]any{}}, "no spec", "bad spec", "u %v"); err == nil {
		t.Error("missing spec accepted")
	}
	if _, err := sharedutils.UnstructuredToStruct[sample](&unstructured.Unstructured{Object: map[string]any{"spec": "notmap"}}, "no spec", "bad spec", "u %v"); err == nil {
		t.Error("non-map spec accepted")
	}
}

func TestResponseCapture(t *testing.T) {
	rec := httptest.NewRecorder()
	rc := sharedutils.NewResponseCapture(rec)
	if rc.Status() != http.StatusOK {
		t.Errorf("default status = %d, want 200", rc.Status())
	}
	rc.WriteHeader(http.StatusTeapot)
	if rc.Status() != http.StatusTeapot {
		t.Errorf("status after WriteHeader = %d", rc.Status())
	}

	rec2 := httptest.NewRecorder()
	rc2 := sharedutils.NewResponseCapture(rec2)
	if _, err := rc2.Write([]byte("body")); err != nil {
		t.Fatal(err)
	}
	if rc2.Status() != http.StatusOK {
		t.Errorf("implicit write status = %d, want 200", rc2.Status())
	}
}

func TestHandleValidationError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"generic", http.ErrNoCookie, http.StatusBadRequest},
		{"challenge expired", newErr(string(constants.ErrChallengeExpired)), http.StatusGone},
		{"session expired", newErr(string(constants.ErrSessionExpired)), http.StatusGone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			sharedutils.HandleValidationError(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestLogAndReturnError(t *testing.T) {
	rec := httptest.NewRecorder()
	sharedutils.LogAndReturnError(rec, http.StatusBadRequest, "boom", http.ErrNoCookie)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestGetHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Token", "abc")
	got, err := sharedutils.GetHeader(httptest.NewRecorder(), r, "X-Token")
	if err != nil || got != "abc" {
		t.Fatalf("GetHeader = %q, err %v", got, err)
	}
	if _, err := sharedutils.GetHeader(httptest.NewRecorder(), r, "X-Missing"); err == nil {
		t.Error("missing header accepted")
	}
}

func TestGetSpec(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	spec, err := sharedutils.GetSpec(httptest.NewRecorder(), r)
	if err != nil || spec["a"] == nil {
		t.Fatalf("GetSpec = %v, err %v", spec, err)
	}
	bad := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{not json`))
	if _, err := sharedutils.GetSpec(httptest.NewRecorder(), bad); err == nil {
		t.Error("invalid body accepted")
	}
}

func TestExtractResourceNameFromRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/a/b/c", nil)
	r = mux.SetURLVars(r, map[string]string{constants.NameParam: "foo"})
	if got := sharedutils.ExtractResourceNameFromRequest(r); got != "foo" {
		t.Errorf("mux var name = %q, want foo", got)
	}
	plain := httptest.NewRequest(http.MethodGet, "/a/b/c", nil)
	if got := sharedutils.ExtractResourceNameFromRequest(plain); got == "" {
		t.Error("path fallback returned empty")
	}
}

func newErr(msg string) error { return &simpleErr{msg} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
