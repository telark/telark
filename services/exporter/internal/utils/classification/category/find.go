package category

import (
	"errors"
	"net/http"

	metadata "github.com/telark/data/metadata/classification"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrCategoryNotFound      = errors.New(string(constants.ErrCategoryNotFound))
	ErrCategoriesCRDNotFound = errors.New(string(constants.ErrCategoriesCRDNotFound))
)

func GetCategoriesCRD() (*unstructured.Unstructured, error) {
	categoriesResource := api.GetCustomResourceByName(constants.CategoriesCRDName, metadata.CategoryAsClassificationMetadata)
	if categoriesResource.Error != nil || categoriesResource.Status != http.StatusOK {
		return nil, ErrCategoriesCRDNotFound
	}

	categories, ok := categoriesResource.Data.(*unstructured.Unstructured)
	if !ok {
		return nil, ErrCategoriesCRDNotFound
	}

	return categories, nil
}

func ExtractCategoriesList(crd *unstructured.Unstructured) ([]map[string]any, error) {
	spec, exists := crd.Object[constants.SpecField].(map[string]any)
	if !exists {
		return nil, errors.New(string(constants.ErrSpecFieldNotFound))
	}

	categories, exists := spec[constants.FieldCategories].([]any)
	if !exists {
		return []map[string]any{}, nil
	}

	result := make([]map[string]any, constants.DefaultInitValue, len(categories))
	for _, cat := range categories {
		if catMap, ok := cat.(map[string]any); ok {
			result = append(result, catMap)
		}
	}

	return result, nil
}

func FindCategoryByID(categoryID string) (map[string]any, error) {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return nil, err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return nil, err
	}

	for _, cat := range categories {
		if id, ok := cat[constants.FieldID].(string); ok && id == categoryID {
			return cat, nil
		}
	}

	return nil, ErrCategoryNotFound
}

func FindCategoriesByScope(scope string) ([]map[string]any, error) {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return nil, err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, constants.DefaultInitValue)
	for _, cat := range categories {
		if catScope, ok := cat[constants.FieldScope].(string); ok && catScope == scope {
			result = append(result, cat)
		}
	}

	return result, nil
}

func GetAllCategories() ([]map[string]any, error) {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return nil, err
	}

	return ExtractCategoriesList(crd)
}

func FindCategoryByIDOrRespond(w http.ResponseWriter, categoryID string) (map[string]any, bool) {
	category, err := FindCategoryByID(categoryID)
	if err != nil {
		handleCategoryError(w, err)
		return nil, false
	}
	return category, true
}

func GetCategoryWithScope(w http.ResponseWriter, categoryID string) (map[string]any, string, bool) {
	category, ok := FindCategoryByIDOrRespond(w, categoryID)
	if !ok {
		return nil, constants.EmptyString, false
	}

	var scope string
	if s, ok := category[constants.FieldScope].(string); ok {
		scope = s
	}

	return category, scope, true
}

func GetAllCategoriesOrRespond(w http.ResponseWriter) ([]map[string]any, bool) {
	categories, err := GetAllCategories()
	if err != nil {
		handleCategoryError(w, err)
		return nil, false
	}
	return categories, true
}

func FindCategoriesByScopeOrRespond(w http.ResponseWriter, scope string) ([]map[string]any, bool) {
	categories, err := FindCategoriesByScope(scope)
	if err != nil {
		handleCategoryError(w, err)
		return nil, false
	}
	return categories, true
}
