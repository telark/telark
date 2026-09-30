package category

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateCategoriesCRDWithFirstCategory(firstCategory map[string]any) error {
	initialSpec := map[string]any{
		constants.FieldCategories: []map[string]any{firstCategory},
	}

	template := sharedutils.ConvertToCRDTemplate(
		metadata.CategoryMetadata,
		constants.CategoriesCRDName,
		initialSpec,
	)

	createResult := api.CreateCustomResourceWithStatus(template, metadata.CategoryMetadata)
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

	patchResult := sharedutils.PatchCustomResource(
		metadata.CategoryMetadata,
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

func UpdateCategoryInCRD(catID string, updatedCategory map[string]any) error {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return err
	}

	i := slices.IndexFunc(categories, hasID(catID))
	if i < constants.DefaultInitValue {
		return errors.New(string(constants.ErrCategoryNotFound))
	}
	categories[i] = updatedCategory

	return UpdateCategoriesInCRD(categories)
}

func DeleteCategoryFromCRD(catID string) error {
	crd, err := GetCategoriesCRD()
	if err != nil {
		return err
	}

	categories, err := ExtractCategoriesList(crd)
	if err != nil {
		return err
	}

	remaining := slices.DeleteFunc(categories, hasID(catID))
	if len(remaining) == len(categories) {
		return errors.New(string(constants.ErrCategoryNotFound))
	}

	return UpdateCategoriesInCRD(remaining)
}
