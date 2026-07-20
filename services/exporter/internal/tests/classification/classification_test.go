package classification

import (
	"net/http/httptest"
	"testing"

	categorydata "github.com/telark/data/classification/category"
	categoryutil "github.com/telark/exporter/internal/utils/classification/category"
)

func TestExtractCategorySpecFromRequestBody(t *testing.T) {
	got, err := categoryutil.ExtractCategorySpecFromRequestBody(map[string]any{"name": "cat", "scope": "roles"})
	if err != nil || got.Name != "cat" || got.Scope != "roles" {
		t.Fatalf("ExtractCategorySpecFromRequestBody = %+v, err %v", got, err)
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
