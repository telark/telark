package cache

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
	rediscache "github.com/telark/x-ware/redis/cache"
)

func GenerateKey(resourceType string, operation string, name string) string {
	if resourceType == constants.EmptyString {
		return ""
	}
	op := strings.ToLower(operation)
	switch op {
	case string(constants.OpList):
		return rediscache.GenerateKey(string(constants.OpList), resourceType, constants.EmptyString)
	case string(constants.OpGet):
		return rediscache.GenerateKey(string(constants.OpGet), resourceType, name)
	default:
		return fmt.Sprintf("%s:%s:%s", op, resourceType, name)
	}
}

func NewListCacheKeyFunc(resourceType string) func(r *http.Request) string {
	return func(_ *http.Request) string {
		return GenerateKey(resourceType, string(constants.OpList), constants.EmptyString)
	}
}

func NewGetCacheKeyFunc(resourceType string) func(r *http.Request) string {
	return func(r *http.Request) string {
		name := sharedutils.ExtractResourceNameFromRequest(r)
		return GenerateKey(resourceType, string(constants.OpGet), name)
	}
}

func GenerateGetKey(endpoint string, name string) string {
	parts := strings.Split(endpoint, "/")
	if len(parts) == constants.DefaultInitValue {
		return ""
	}
	resourceType := parts[len(parts)-constants.IndexLastElementOffset]
	return GenerateKey(resourceType, string(constants.OpGet), name)
}

func ValidateCacheKey(key string) bool {
	return rediscache.ValidateKey(key)
}
