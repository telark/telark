package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	sharedexp "github.com/telark/exporter/exporters/shared"
	"github.com/telark/exporter/utils/performance"
	sharedutils "github.com/telark/exporter/utils/shared"
)

func CreateResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
	linkFunc func(http.ResponseWriter, *http.Request, string),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		spec, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}
		resourceName := sharedutils.ExtractResourceNameFromRequestBody(spec)
		if resourceName == "" {
			sharedutils.LogAndReturnError(w, http.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil)
			return
		}
		sharedexp.CreateResource(w, md, resourceName, spec)

		if linkFunc != nil {
			linkFunc(w, r, resourceName)
		}

		cache.SmartInvalidateListCache(optimizer, resourceType, string(constants.OpCreate))
		cache.InvalidateAllResourceCaches(optimizer, resourceType)
	}
}

func GetResourceWithCacheInvalidation(
	_ *performance.Optimizer,
	md metadata.Metadata,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.GetOrListResource(w, r, md, constants.OpGet)
	}
}

func GetUniqueResourceFromListWithCacheInvalidation(
	md metadata.Metadata,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.GetUniqueResourceFromList(w, r, md)
	}
}

func ListResourceWithCacheInvalidation(
	_ *performance.Optimizer,
	md metadata.Metadata,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.GetOrListResource(w, r, md, constants.OpList)
	}
}

func PatchResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
	linkFunc func(http.ResponseWriter, *http.Request),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.PatchResource(w, r, md)

		// Run cascade function if provided
		if linkFunc != nil {
			linkFunc(w, r)
		}

		resourceName := sharedutils.ExtractResourceNameFromRequest(r)
		if resourceName != constants.EmptyString {
			cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
		} else {
			cache.SmartInvalidateListCache(optimizer, resourceType, string(constants.OpPatch))
			cache.InvalidateAllResourceCaches(optimizer, resourceType)
		}
	}
}

func DeleteResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
	linkFunc func(http.ResponseWriter, *http.Request, metadata.Metadata, string),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceName, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
			sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
			return
		}

		if linkFunc != nil {
			linkFunc(w, r, md, resourceName)
		} else {
			sharedexp.DeleteResource(w, r, md, resourceName)
		}

		if resourceName != constants.EmptyString {
			cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
		} else {
			cache.SmartInvalidateListCache(optimizer, resourceType, string(constants.OpDelete))
			cache.InvalidateAllResourceCaches(optimizer, resourceType)
		}
	}
}
