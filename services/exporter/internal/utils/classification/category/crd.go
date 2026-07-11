package category

import (
	"errors"
	"fmt"
	"net/http"

	metadata "github.com/telark/data/metadata/classification"
	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
)

func CreateCategoriesCRDWithFirstCategory(firstCategory map[string]any) error {
	initialSpec := map[string]any{
		constants.FieldCategories: []map[string]any{firstCategory},
	}

	template := sharedutils.ConvertToCRDTemplate(
		metadata.CategoryAsClassificationMetadata,
		constants.CategoriesCRDName,
		initialSpec,
	)

	createResult := api.CreateCustomResource(template, metadata.CategoryAsClassificationMetadata)
	if createResult.Error != nil || createResult.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedToCreateCategoriesCRD), createResult.Error)
	}

	return nil
}

func UpdateCategoriesInCRD(categories []map[string]any) error {
	spec := map[string]any{
		constants.FieldCategories: categories,
	}

	specPatchData := map[string]any{
		constants.SpecField: spec,
	}

	patchResult := api.PatchCustomResource(
		metadata.CategoryAsClassificationMetadata,
		constants.CategoriesCRDName,
		specPatchData,
	)

	if patchResult.Error != nil || patchResult.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrFailedToUpdateCategoriesCRD), patchResult.Error)
	}

	return nil
}

func AddCategoryToCRD(newCategory map[string]any) error {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return CreateCategoriesCRDWithFirstCategory(newCategory)
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return err
	}
	categories = append(categories, newCategory)

	return UpdateCategoriesInCRD(categories)
}

func UpdateCategoryInCRD(categoryID string, updatedCategory map[string]any) error {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return err
	}

	found := false
	for i, cat := range categories {
		if id, ok := cat[constants.FieldID].(string); ok && id == categoryID {
			categories[i] = updatedCategory
			found = true
			break
		}
	}

	if !found {
		return errors.New(string(constants.ErrCategoryNotFound))
	}

	return UpdateCategoriesInCRD(categories)
}

func DeleteCategoryFromCRD(categoryID string) error {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return err
	}

	found := false
	newCategories := make([]map[string]any, constants.DefaultInitValue, len(categories))
	for _, cat := range categories {
		if id, ok := cat[constants.FieldID].(string); ok && id == categoryID {
			found = true
			continue
		}
		newCategories = append(newCategories, cat)
	}

	if !found {
		return errors.New(string(constants.ErrCategoryNotFound))
	}

	return UpdateCategoriesInCRD(newCategories)
}
