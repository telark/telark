package categories

import (
	"fmt"
	"maps"
	"net/http"

	categorydata "github.com/telark/telark/internal/data/classification/category"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	categoryendpoints "github.com/telark/telark/internal/rest/endpoints/categories"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	categoryutils "github.com/telark/telark/services/exporter/internal/utils/classification/category"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateCategoryResourceWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := sharedutils.GetSpecFor[categorydata.Category](w, r)
		if err != nil {
			return
		}

		category, err := categoryutils.ExtractCategorySpecFromRequestBody(body)
		if err != nil {
			responseutils.SendResponse(
				w,
				http.StatusBadRequest,
				response.OperationError,
				err.Error(),
				nil,
			)
			return
		}

		if err := categoryutils.ValidateAndPrepareCategory(category, w); err != nil {
			return
		}

		if !authz.GuardCategoryScope(w, r, category.Scope, constants.CategoryOpCreate) {
			return
		}

		if !rejectBuiltinType(w, string(category.Type)) {
			return
		}

		createCategoryResource(w, category, optimizer)
	}
}

func rejectBuiltinType(w http.ResponseWriter, categoryType string) bool {
	if categoryType != string(categorydata.CategoryTypeBuiltIn) {
		return true
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusBadRequest,
		response.OperationError,
		string(constants.ErrCategoryBuiltInTypeReserved),
		nil,
		nil,
	)
	return false
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

	if !categoryutils.EnsureNameAvailable(w, category.Scope, category.Name, category.ID) {
		return
	}

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

	msg := fmt.Sprintf(string(messages.SuccessCreateRes), category.ID, metadata.CategoryMetadata.Kind)
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
		catID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		category, ok := categoryutils.FindCategoryByIDOrRespond(w, catID)
		if !ok {
			return
		}

		msg := fmt.Sprintf(string(messages.SuccessGetRes), catID, metadata.CategoryMetadata.Kind)
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

// ?scope= narrows the list to one scope; the cache key carries the value.
func ListCategoriesWithCacheInvalidation() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			categories []map[string]any
			ok         bool
		)
		if scope := r.URL.Query().Get(categoryendpoints.QueryScope); scope != constants.EmptyString {
			categories, ok = categoryutils.FindCategoriesByScopeOrRespond(w, scope)
		} else {
			categories, ok = categoryutils.GetAllCategoriesOrRespond(w)
		}
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
	catID string,
	existingCategory map[string]any,
	body map[string]any,
	optimizer *performance.Optimizer,
) {
	resourcesshared.AddLastUpdateDateToPatchBody(body)

	updatedCategory := make(map[string]any)
	maps.Copy(updatedCategory, existingCategory)
	maps.Copy(updatedCategory, body)

	updatedCategory[constants.FieldID] = catID

	lock := concurrency.GetLock(constants.CategoriesCRDName)
	lock.Lock()
	defer lock.Unlock()

	scope := categoryutils.StringField(updatedCategory, constants.FieldScope)
	name := categoryutils.StringField(updatedCategory, constants.FieldName)
	if !categoryutils.EnsureNameAvailable(w, scope, name, catID) {
		return
	}

	if err := categoryutils.UpdateCategoryInCRD(catID, updatedCategory); err != nil {
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
	cache.InvalidateGetCache(optimizer, constants.ResourceCategory, catID)

	msg := fmt.Sprintf(string(messages.SuccessUpdateRes), catID, metadata.CategoryMetadata.Kind)
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
		catID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		existingCategory, oldScope, ok := categoryutils.GetCategoryWithScope(w, catID)
		if !ok {
			return
		}

		if !authz.GuardCategoryScope(w, r, oldScope, constants.CategoryOpEdit) {
			return
		}

		body, err := sharedutils.GetSpecFor[categorydata.Category](w, r)
		if err != nil {
			return
		}

		if !rejectBuiltin(w, catID) || !rejectBuiltinType(w, categoryutils.StringField(body, constants.FieldType)) {
			return
		}

		newScope := categoryutils.StringField(body, constants.FieldScope)

		// Moving a category between scopes needs the same right on the scope it
		// is moving into, or it would be a way to write into a scope the caller
		// does not hold.
		if newScope != constants.EmptyString && newScope != oldScope {
			if !authz.GuardCategoryScope(w, r, newScope, constants.CategoryOpEdit) {
				return
			}
		}

		patchCategoryResource(w, catID, existingCategory, body, optimizer)
	}
}

// The UI hides edit and delete for built-ins; the seed restores them anyway.
func rejectBuiltin(w http.ResponseWriter, catID string) bool {
	if !categoryutils.IsBuiltinCategory(catID) {
		return true
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusBadRequest,
		response.OperationError,
		string(constants.ErrCategoryBuiltInImmutable),
		nil,
		nil,
	)
	return false
}

func DeleteCategoryByIDWithCacheInvalidation(optimizer *performance.Optimizer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		catID, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}

		_, deletedScope, ok := categoryutils.GetCategoryWithScope(w, catID)
		if !ok {
			return
		}

		if !authz.GuardCategoryScope(w, r, deletedScope, constants.CategoryOpDelete) {
			return
		}

		if !rejectBuiltin(w, catID) {
			return
		}

		lock := concurrency.GetLock(constants.CategoriesCRDName)
		lock.Lock()
		defer lock.Unlock()

		if !categoryutils.ValidateCategoryDeletion(w) {
			return
		}

		if err := categoryutils.DeleteCategoryFromCRD(catID); err != nil {
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
		cache.InvalidateGetCache(optimizer, constants.ResourceCategory, catID)

		msg := fmt.Sprintf(string(messages.SuccessDeleteRes), catID, metadata.CategoryMetadata.Kind)
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
