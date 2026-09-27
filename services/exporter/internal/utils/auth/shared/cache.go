package shared

import (
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
)

func InvalidateResourceCaches(optimizer *performance.Optimizer, resourceType string, operation string, resourceName string) {
	if resourceName != constants.EmptyString {
		cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceName)
		return
	}
	cache.SmartInvalidateListCache(optimizer, resourceType, operation)
}
