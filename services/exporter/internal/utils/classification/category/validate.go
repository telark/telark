package category

import (
	"net/http"

	categorydata "github.com/telark/data/classification/category"
	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func ValidateCategoryDeletion(w http.ResponseWriter) bool {
	categories, err := GetAllCategories()
	if err != nil {
		handleCategoryError(w, err)
		return false
	}

	if len(categories) <= constants.MinCategoriesInCRD {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrCategoryCannotDeleteLast),
			nil,
			nil,
		)
		return false
	}

	return true
}

func validateRequiredFieldAndRespond(w http.ResponseWriter, value string, errorMsg string) error {
	if err := sharedutils.ValidateRequiredField(value, errorMsg); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return err
	}
	return nil
}

func ValidateAndPrepareCategory(category *categorydata.Category, w http.ResponseWriter) error {
	if err := validateRequiredFieldAndRespond(w, category.Name, string(constants.ErrCategoryNameCannotBeEmpty)); err != nil {
		return err
	}

	if err := validateRequiredFieldAndRespond(w, category.Scope, string(constants.ErrCategoryScopeCannotBeEmpty)); err != nil {
		return err
	}

	categoryID, err := GenerateUniqueCategoryID()
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return err
	}
	category.ID = categoryID

	return nil
}
