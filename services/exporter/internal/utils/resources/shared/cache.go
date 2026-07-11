package shared

import (
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/utils/performance"
)

func InvalidateResourceCaches(optimizer *performance.Optimizer, resourceType string, resourceID string) {
	cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceID)
}
