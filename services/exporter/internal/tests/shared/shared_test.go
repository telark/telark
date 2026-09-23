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

const (
	testAppName = "app-1"

	valueA   = "a"
	valueX   = "x"
	valueY   = "y"
	rootPath = "/"

	notMapSpec = "notmap"
	errNoSpec  = "no spec"
	errBadSpec = "bad spec"
)

func TestValidateRequiredField(t *testing.T) {
	if err := sharedutils.ValidateRequiredField(constants.EmptyString, "required"); err == nil {
		t.Error("empty value accepted")
	}
	if err := sharedutils.ValidateRequiredField(valueX, "required"); err != nil {
		t.Errorf("non-empty value rejected: %v", err)
	}
}

func TestGenerateResourceError(t *testing.T) {
	format := dataerrors.Error("resource %s failed: %v")
	withErr := sharedutils.GenerateResourceError(format, testAppName, http.ErrNoCookie)
	if !strings.Contains(withErr, testAppName) {
		t.Errorf("missing resource name: %q", withErr)
	}
	// A nil cause still yields a fully formatted message via the unknown-error fallback.
	if got := sharedutils.GenerateResourceError(format, testAppName, nil); !strings.Contains(got, testAppName) {
		t.Errorf("nil-error path lost the resource name: %q", got)
	}
}

func TestExtractResourceNameFromRequestBody(t *testing.T) {
	if got := sharedutils.ExtractResourceNameFromRequestBody(map[string]any{constants.NameParam: valueX}); got != valueX {
		t.Errorf("got %q, want x", got)
	}
	if got := sharedutils.ExtractResourceNameFromRequestBody(map[string]any{}); got != constants.EmptyString {
		t.Errorf("missing name should be empty, got %q", got)
	}
}

func TestExtractMapValue(t *testing.T) {
	data := map[string]any{"m": map[string]any{"k": constants.DefaultIncrementValue}, "s": valueX}
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
	body := map[string]any{constants.IDParam: valueX}
	sharedutils.RemoveIDFromRequestBody(body)
	if _, ok := body[constants.IDParam]; ok {
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
	got, err := sharedutils.ExtractStructFromBody[sample](map[string]any{constants.NameParam: valueX})
	if err != nil || got.Name != valueX {
		t.Fatalf("ExtractStructFromBody = %+v, err %v", got, err)
	}
	body := map[string]any{constants.IDParam: "drop", constants.NameParam: valueY}
	got, err = sharedutils.ExtractStructFromBodyIgnoringID[sample](body)
	if err != nil || got.Name != valueY {
		t.Fatalf("ExtractStructFromBodyIgnoringID = %+v, err %v", got, err)
	}
	if _, ok := body[constants.IDParam]; ok {
		t.Error("id not stripped before extraction")
	}
}

func TestFilterDataSingleItem(t *testing.T) {
	item := &unstructured.Unstructured{Object: map[string]any{
		constants.SpecField: map[string]any{"field": "v"},
		constants.MetadataField: map[string]any{
			"resourceVersion":   "12",
			constants.NameParam: testAppName,
			"extra":             "dropped",
		},
	}}
	out, err := sharedutils.FilterData(item)
	if err != nil {
		t.Fatalf("FilterData: %v", err)
	}
	res, ok := out.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("FilterData returned %T, want a single resource", out)
	}
	if res.Object["field"] != "v" {
		t.Error("spec field not surfaced")
	}
	meta, ok := res.Object[constants.MetadataField].(map[string]any)
	if !ok {
		t.Fatalf("filtered metadata is %T, want a map", res.Object[constants.MetadataField])
	}
	if meta["resourceVersion"] != "12" || meta[constants.NameParam] != testAppName {
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
	if _, err := sharedutils.FilterData(&unstructured.Unstructured{Object: map[string]any{constants.SpecField: notMapSpec}}); err == nil {
		t.Error("non-map spec accepted")
	}
}

func TestFilterDataList(t *testing.T) {
	list := &unstructured.UnstructuredList{Items: []unstructured.Unstructured{
		{Object: map[string]any{constants.SpecField: map[string]any{valueA: constants.DefaultIncrementValue}}},
		{Object: map[string]any{"nospec": true}},
		{Object: map[string]any{constants.SpecField: notMapSpec}},
	}}
	out, err := sharedutils.FilterData(list)
	if err != nil {
		t.Fatalf("FilterData list: %v", err)
	}
	filtered, ok := out.(*unstructured.UnstructuredList)
	if !ok {
		t.Fatalf("FilterData returned %T, want a list", out)
	}
	if len(filtered.Items) != constants.DefaultIncrementValue {
		t.Errorf("expected 1 valid item, got %d", len(filtered.Items))
	}
}

func TestFilterResourceOrRespond(t *testing.T) {
	valid := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{valueA: constants.DefaultIncrementValue}}}
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
	tmpl := sharedutils.ConvertToCRDTemplate(md, testAppName, map[string]any{valueX: constants.DefaultIncrementValue})
	if tmpl.Object["apiVersion"] != "erpi.telark/v1alpha1" || tmpl.Object["kind"] != "Role" {
		t.Errorf("bad envelope: %v", tmpl.Object)
	}
	withFin := sharedutils.ConvertToCRDTemplateWithFinalizers(md, testAppName, map[string]any{}, []string{"f1"})
	meta, ok := withFin.Object[constants.MetadataField].(map[string]any)
	if !ok {
		t.Fatalf("template metadata is %T, want a map", withFin.Object[constants.MetadataField])
	}
	if meta["finalizers"] == nil {
		t.Error("finalizers not attached")
	}
	noFin := sharedutils.ConvertToCRDTemplateWithFinalizers(md, testAppName, map[string]any{}, nil)
	noFinMeta, ok := noFin.Object[constants.MetadataField].(map[string]any)
	if !ok {
		t.Fatalf("template metadata is %T, want a map", noFin.Object[constants.MetadataField])
	}
	if _, ok := noFinMeta["finalizers"]; ok {
		t.Error("empty finalizers should be omitted")
	}
	bare := sharedutils.ConvertToUnstructuredWithoutManagedFields(map[string]any{valueA: constants.DefaultIncrementValue})
	if bare.Object[constants.SpecField] == nil {
		t.Error("spec missing from bare unstructured")
	}
}

func TestStructAndUnstructuredConversion(t *testing.T) {
	spec, err := sharedutils.StructToSpecMap(sample{Name: valueX})
	if err != nil || spec[constants.NameParam] != valueX {
		t.Fatalf("StructToSpecMap = %v, err %v", spec, err)
	}
	res := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: map[string]any{constants.NameParam: valueY}}}
	got, err := sharedutils.UnstructuredToStruct[sample](res, errNoSpec, errBadSpec, "unmarshal %v")
	if err != nil || got.Name != valueY {
		t.Fatalf("UnstructuredToStruct = %+v, err %v", got, err)
	}
	noSpec := &unstructured.Unstructured{Object: map[string]any{}}
	if _, err := sharedutils.UnstructuredToStruct[sample](noSpec, errNoSpec, errBadSpec, "u %v"); err == nil {
		t.Error("missing spec accepted")
	}
	badSpec := &unstructured.Unstructured{Object: map[string]any{constants.SpecField: notMapSpec}}
	if _, err := sharedutils.UnstructuredToStruct[sample](badSpec, errNoSpec, errBadSpec, "u %v"); err == nil {
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
	r := httptest.NewRequest(http.MethodGet, rootPath, nil)
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
	r := httptest.NewRequest(http.MethodPost, rootPath, strings.NewReader(`{"a":1}`))
	spec, err := sharedutils.GetSpec(httptest.NewRecorder(), r)
	if err != nil || spec[valueA] == nil {
		t.Fatalf("GetSpec = %v, err %v", spec, err)
	}
	bad := httptest.NewRequest(http.MethodPost, rootPath, strings.NewReader(`{not json`))
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
	if got := sharedutils.ExtractResourceNameFromRequest(plain); got == constants.EmptyString {
		t.Error("path fallback returned empty")
	}
}

func newErr(msg string) error { return &simpleErr{msg} }

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
