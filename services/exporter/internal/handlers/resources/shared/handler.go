package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedexp "github.com/telark/telark/services/exporter/internal/exporters/shared"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		spec, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}
		resourceName := sharedutils.ExtractResourceNameFromRequestBody(spec)
		if resourceName == constants.EmptyString {
			sharedutils.LogAndReturnError(w, http.StatusBadRequest, string(errors.ErrResourceNameCannotBeEmpty), nil)
			return
		}
		sharedexp.CreateResource(w, md, resourceName, spec)

		cache.InvalidateAllResourceCaches(optimizer, resourceType)
	}
}

func GetResourceWithCacheInvalidation(md metadata.Metadata) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.GetOrListResource(w, r, md, constants.OpGet)
	}
}

func ListResourceWithCacheInvalidation(md metadata.Metadata) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sharedexp.GetOrListResource(w, r, md, constants.OpList)
	}
}

func PatchResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceName := sharedutils.ExtractResourceNameFromRequest(r)
		// The response goes out before the invalidation below; a client that reads
		// back immediately must not get the pre-write copy.
		if resourceName != constants.EmptyString {
			cache.InvalidateGetCache(optimizer, resourceType, resourceName)
		}
		sharedexp.PatchResource(w, r, md)

		if resourceName != constants.EmptyString {
			cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
		} else {
			cache.InvalidateAllResourceCaches(optimizer, resourceType)
		}
	}
}

func DeleteResourceWithCacheInvalidation(
	optimizer *performance.Optimizer,
	md metadata.Metadata,
	resourceType string,
	deleteFunc func(http.ResponseWriter, metadata.Metadata, string),
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		resourceName, err := sharedutils.GetPathParam(w, r, constants.NameParam)
		if err != nil {
			msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
			sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
			return
		}

		deleteFunc(w, md, resourceName)

		if resourceName != constants.EmptyString {
			cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
		} else {
			cache.InvalidateAllResourceCaches(optimizer, resourceType)
		}
	}
}
