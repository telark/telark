package cache

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	rediscache "github.com/telark/x-ware/redis/cache"
)

type SubjectFunc func(r *http.Request) string

type ListGenerationReader interface {
	ListGeneration(resourceType string) string
}

func GenerateKey(resourceType string, operation string, name string) string {
	if resourceType == constants.EmptyString {
		return constants.EmptyString
	}
	return rediscache.GenerateKey(operation, resourceType, name)
}

func ListGenerationKey(resourceType string) string {
	return rediscache.BuildKey(constants.CacheKeyPrefix, constants.OpList, constants.CacheGenerationSegment, resourceType)
}

// ListGenerationWindowKey exists while a bump window is open; ListGenerationDirtyKey
// records a bump deferred by that window.
func ListGenerationWindowKey(resourceType string) string {
	return ListGenerationKey(resourceType) + constants.CacheKeySeparator + constants.CacheWindowSegment
}

func ListGenerationDirtyKey(resourceType string) string {
	return ListGenerationKey(resourceType) + constants.CacheKeySeparator + constants.CacheDirtySegment
}

func ListKeyPattern(resourceType string) string {
	return rediscache.BuildKey(constants.OpList, resourceType) + constants.CacheKeySeparator + "*"
}

func generateListKey(resourceType string, generation string, subject string) string {
	if resourceType == constants.EmptyString {
		return constants.EmptyString
	}
	return rediscache.BuildKey(constants.OpList, resourceType, generation, subject)
}

func NewListCacheKeyFunc(generations ListGenerationReader, resourceType string) func(r *http.Request) string {
	return func(_ *http.Request) string {
		return generateListKey(resourceType, generations.ListGeneration(resourceType), constants.EmptyString)
	}
}

// The summary view is a pruned blob: served to a full-view caller it would read
// as an application with no snapshots and no history, so the view is in the key.
func NewViewListCacheKeyFunc(generations ListGenerationReader, resourceType string) func(r *http.Request) string {
	return func(r *http.Request) string {
		view := r.URL.Query().Get(constants.ViewParam)
		if view != constants.ViewSummary {
			view = constants.ViewFull
		}
		return generateListKey(resourceType, generations.ListGeneration(resourceType), view)
	}
}

// A subject-less key would be shared by every caller of the route, so a request
// whose subject cannot be read gets no key and therefore bypasses the cache.
func NewSubjectListCacheKeyFunc(
	generations ListGenerationReader,
	resourceType string,
	subject SubjectFunc,
) func(r *http.Request) string {
	return func(r *http.Request) string {
		subjectID := subject(r)
		if subjectID == constants.EmptyString {
			return constants.EmptyString
		}
		return generateListKey(resourceType, generations.ListGeneration(resourceType), subjectID)
	}
}

func SubjectFromPathParam(param string) SubjectFunc {
	return func(r *http.Request) string {
		return subjectSegment(param, mux.Vars(r)[param])
	}
}

func SubjectFromHeader(header string) SubjectFunc {
	return func(r *http.Request) string {
		return subjectSegment(header, r.Header.Get(header))
	}
}

// The source is part of the segment so that two subjects of different kinds,
// such as a user id and a group id, cannot land on the same key.
func subjectSegment(label string, value string) string {
	if value == constants.EmptyString {
		return constants.EmptyString
	}
	return rediscache.BuildKey(label, value)
}

func NewGetCacheKeyFunc(resourceType string) func(r *http.Request) string {
	return func(r *http.Request) string {
		name := sharedutils.ExtractResourceNameFromRequest(r)
		return GenerateKey(resourceType, constants.OpGet, name)
	}
}

func GenerateGetKey(endpoint string, name string) string {
	parts := strings.Split(endpoint, "/")
	if len(parts) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	resourceType := parts[len(parts)-constants.IndexLastElementOffset]
	return GenerateKey(resourceType, constants.OpGet, name)
}

func ValidateCacheKey(key string) bool {
	return rediscache.ValidateKey(key)
}
