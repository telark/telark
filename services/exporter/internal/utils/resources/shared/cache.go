package shared

import (
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/utils/performance"
)

func InvalidateResourceCaches(optimizer *performance.Optimizer, resourceType string, resourceID string) {
	cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceID)
}
