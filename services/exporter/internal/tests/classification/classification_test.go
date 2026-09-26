package classification

import (
	"net/http/httptest"
	"testing"

	categorydata "github.com/telark/data/classification/category"
	"github.com/telark/exporter/internal/constants"
	categoryutil "github.com/telark/exporter/internal/utils/classification/category"
)

func TestExtractCategorySpecFromRequestBody(t *testing.T) {
	got, err := categoryutil.ExtractCategorySpecFromRequestBody(map[string]any{"name": "cat", "scope": "roles"})
	if err != nil || got.Name != "cat" || got.Scope != "roles" {
		t.Fatalf("ExtractCategorySpecFromRequestBody = %+v, err %v", got, err)
	}
}

func TestIsBuiltinCategory(t *testing.T) {
	if !categoryutil.IsBuiltinCategory(categorydata.BuiltinCategories[0].ID) {
		t.Error("seeded built-in not recognized")
	}
	if categoryutil.IsBuiltinCategory("cat-custom-0001-0001") {
		t.Error("custom id reported as built-in")
	}
}

// The UI enforces case-insensitive uniqueness per scope; the server has to
// agree or a raw API call can create the duplicate the UI hides.
func TestNameTaken(t *testing.T) {
	const production = "Production"
	categories := []map[string]any{
		{"id": "cat-1", "scope": categorydata.ScopePlanEnvironments, "name": production},
		{"id": "cat-2", "scope": categorydata.ScopePlanTags, "name": "Security"},
	}
	tests := []struct {
		name, scope, candidate, excludeID string
		want                              bool
	}{
		{"exact", categorydata.ScopePlanEnvironments, production, constants.EmptyString, true},
		{"case and spaces", categorydata.ScopePlanEnvironments, "  production ", constants.EmptyString, true},
		{"other scope", categorydata.ScopePlanTags, production, constants.EmptyString, false},
		{"own id on rename", categorydata.ScopePlanEnvironments, production, "cat-1", false},
		{"free name", categorydata.ScopePlanEnvironments, "Staging", constants.EmptyString, false},
	}
	for _, tt := range tests {
		if got := categoryutil.NameTaken(categories, tt.scope, tt.candidate, tt.excludeID); got != tt.want {
			t.Errorf("%s: NameTaken = %v, want %v", tt.name, got, tt.want)
		}
	}
}

const (
	testCategoryName  = "cat"
	testCategoryScope = "roles"
)

// A create without type reached the CRD as "" and failed with a raw 500.
func TestValidateAndPrepareCategoryDefaultsType(t *testing.T) {
	tests := []struct {
		name string
		in   categorydata.CategoryType
		want categorydata.CategoryType
	}{
		{"missing type", categorydata.CategoryType(constants.EmptyString), categorydata.CategoryTypeCustom},
		{"custom kept", categorydata.CategoryTypeCustom, categorydata.CategoryTypeCustom},
		{"built-in kept for the reserved check", categorydata.CategoryTypeBuiltIn, categorydata.CategoryTypeBuiltIn},
	}
	for _, tt := range tests {
		category := &categorydata.Category{Name: testCategoryName, Scope: testCategoryScope, Type: tt.in}
		// No cluster in tests: the ID lookup may fail after the type is settled.
		_ = categoryutil.ValidateAndPrepareCategory(category, httptest.NewRecorder())
		if category.Type != tt.want {
			t.Errorf("%s: Type = %q, want %q", tt.name, category.Type, tt.want)
		}
	}
}

func TestValidateAndPrepareCategoryFailures(t *testing.T) {
	tests := []struct {
		name     string
		category *categorydata.Category
	}{
		{"empty name", &categorydata.Category{Scope: "roles"}},
		{"empty scope", &categorydata.Category{Name: "cat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if err := categoryutil.ValidateAndPrepareCategory(tt.category, rec); err == nil {
				t.Errorf("%s: expected validation error", tt.name)
			}
		})
	}
}
