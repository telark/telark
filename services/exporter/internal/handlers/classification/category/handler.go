package category

import (
	"fmt"
	"maps"
	"net/http"

	categorydata "github.com/telark/data/classification/category"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/classification"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	categoryutils "github.com/telark/exporter/internal/utils/classification/category"
	"github.com/telark/exporter/internal/utils/concurrency"
	"github.com/telark/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateCategoryResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		category, err := categoryutils.ExtractCategorySpecFromRequestBody(body)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusBadRequest,
				response.OperationError,
				err.Error(),
				nil,
				err,
			)
			return
		}

		if err := categoryutils.ValidateAndPrepareCategory(category, w); err != nil {
			return
		}

		if !authz.GuardCategoryScope(w, r, category.Scope, constants.CategoryOpCreate) {
			return
		}

		createCategoryResource(w, category, optimizer)
	}
}

func createCategoryResource(w http.ResponseWriter, category *categorydata.Category, optimizer *performance.Optimizer) {
	categoryMap := map[string]any{
		constants.FieldID:           category.ID,
		constants.FieldName:         category.Name,
		constants.FieldScope:        category.Scope,
		constants.FieldType:         string(category.Type),
		constants.FieldCreationDate: category.CreationDate,
	}

	lock := concurrency.GetLock(constants.CategoriesCRDName)
	lock.Lock()
	defer lock.Unlock()

	if err := categoryutils.AddCategoryToCRD(categoryMap); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return
	}

	categoryutils.InvalidateCategoryCaches(optimizer)

	msg := fmt.Sprintf(string(messages.SuccessCreateRes), category.ID, metadata.CategoryAsClassificationMetadata.Kind)
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		categoryMap,
		nil,
	)
}

func GetCategoryByIDWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		category, ok := categoryutils.FindCategoryByIDOrRespond(w, categoryID)
		if !ok {
			return
		}

		msg := fmt.Sprintf(string(messages.SuccessGetRes), categoryID, metadata.CategoryAsClassificationMetadata.Kind)
		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			msg,
			category,
			nil,
		)
	}
}

func ListAllCategoriesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		categories, ok := categoryutils.GetAllCategoriesOrRespond(w)
		if !ok {
			return
		}

		responseData := map[string]any{
			constants.FieldItems: categories,
		}

		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			string(messages.SuccessGetRes),
			responseData,
			nil,
		)
	}
}

func GetCategoriesByScopeWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		scope, err := sharedutils.GetPathParam(w, r, constants.FieldScope)
		if err != nil {
			return
		}

		categories, ok := categoryutils.FindCategoriesByScopeOrRespond(w, scope)
		if !ok {
			return
		}

		responseData := map[string]any{
			constants.FieldItems: categories,
		}

		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			string(messages.SuccessGetRes),
			responseData,
			nil,
		)
	}
}

func patchCategoryResource(
	w http.ResponseWriter,
	categoryID string,
	existingCategory map[string]any,
	body map[string]any,
	optimizer *performance.Optimizer,
) {
	resourcesshared.AddLastUpdateDateToPatchBody(body)

	updatedCategory := make(map[string]any)
	maps.Copy(updatedCategory, existingCategory)
	maps.Copy(updatedCategory, body)

	updatedCategory[constants.FieldID] = categoryID

	lock := concurrency.GetLock(constants.CategoriesCRDName)
	lock.Lock()
	defer lock.Unlock()

	if err := categoryutils.UpdateCategoryInCRD(categoryID, updatedCategory); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return
	}

	categoryutils.InvalidateCategoryCaches(optimizer)
	cache.InvalidateGetCache(optimizer, constants.ResourceCategory, categoryID)

	msg := fmt.Sprintf(string(messages.SuccessUpdateRes), categoryID, metadata.CategoryAsClassificationMetadata.Kind)
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		updatedCategory,
		nil,
	)
}

func PatchCategoryByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingCategory, oldScope, ok := categoryutils.GetCategoryWithScope(w, categoryID)
		if !ok {
			return
		}

		if !authz.GuardCategoryScope(w, r, oldScope, constants.CategoryOpEdit) {
			return
		}

		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}

		var newScope string
		if scope, ok := body[constants.FieldScope].(string); ok {
			newScope = scope
		}

		// Moving a category between scopes needs the same right on the scope it
		// is moving into, or it would be a way to write into a scope the caller
		// does not hold.
		if newScope != constants.EmptyString && newScope != oldScope {
			if !authz.GuardCategoryScope(w, r, newScope, constants.CategoryOpEdit) {
				return
			}
		}

		patchCategoryResource(w, categoryID, existingCategory, body, optimizer)

		if oldScope != constants.EmptyString && oldScope != newScope {
			cache.InvalidateGetCache(optimizer, constants.ResourceCategory, oldScope)
		}
	}
}

func DeleteCategoryByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		_, deletedScope, ok := categoryutils.GetCategoryWithScope(w, categoryID)
		if !ok {
			return
		}

		if !authz.GuardCategoryScope(w, r, deletedScope, constants.CategoryOpDelete) {
			return
		}

		lock := concurrency.GetLock(constants.CategoriesCRDName)
		lock.Lock()
		defer lock.Unlock()

		if !categoryutils.ValidateCategoryDeletion(w) {
			return
		}

		if err := categoryutils.DeleteCategoryFromCRD(categoryID); err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusInternalServerError,
				response.OperationError,
				err.Error(),
				nil,
				err,
			)
			return
		}

		categoryutils.InvalidateCategoryCaches(optimizer)
		cache.InvalidateGetCache(optimizer, constants.ResourceCategory, categoryID)

		if deletedScope != constants.EmptyString {
			cache.InvalidateGetCache(optimizer, constants.ResourceCategory, deletedScope)
		}

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), categoryID, metadata.CategoryAsClassificationMetadata.Kind)
		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			msg,
			nil,
			nil,
		)
	}
}
