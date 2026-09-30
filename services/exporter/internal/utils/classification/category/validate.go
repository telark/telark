package category

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	categorydata "github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func IsBuiltinCategory(id string) bool {
	return slices.ContainsFunc(categorydata.BuiltinCategories, func(c categorydata.Category) bool { return c.ID == id })
}

func StringField(category map[string]any, key string) string {
	value, isString := category[key].(string)
	if !isString {
		return constants.EmptyString
	}
	return value
}

// Names compare trimmed and case-insensitively within a scope, as the UI does.
func NameTaken(categories []map[string]any, scope, name, excludeID string) bool {
	want := strings.ToLower(strings.TrimSpace(name))
	return slices.ContainsFunc(categories, func(c map[string]any) bool {
		return StringField(c, constants.FieldID) != excludeID &&
			StringField(c, constants.FieldScope) == scope &&
			strings.ToLower(strings.TrimSpace(StringField(c, constants.FieldName))) == want
	})
}

// Called under the categories lock so two parallel creates cannot both pass.
// A missing CRD holds no names: the first create builds it.
func EnsureNameAvailable(w http.ResponseWriter, scope, name, excludeID string) bool {
	categories, err := GetAllCategories()
	if err != nil && !errors.Is(err, ErrCategoriesCRDNotFound) {
		handleCategoryError(w, err)
		return false
	}
	if NameTaken(categories, scope, name, excludeID) {
		responseutils.LogAndSendResponse(
			w,
			http.StatusConflict,
			response.OperationAlreadyExists,
			string(constants.ErrCategoryNameAlreadyExists),
			nil,
			nil,
		)
		return false
	}
	return true
}

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

	if category.Type == categorydata.CategoryType(constants.EmptyString) {
		category.Type = categorydata.CategoryTypeCustom
	}

	catID, err := GenerateUniqueCategoryID()
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
	category.ID = catID

	return nil
}
