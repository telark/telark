package shared

import (
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/utils/performance"
)

func InvalidateResourceCaches(optimizer *performance.Optimizer, resourceType string, operation string, resourceName string) {
	if resourceName != constants.EmptyString {
		cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
	}
	cache.SmartInvalidateListCache(optimizer, resourceType, operation)
	cache.InvalidateAllResourceCaches(optimizer, resourceType)
}
