package shared

import (
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
)

func InvalidateResourceCaches(optimizer *performance.Optimizer, resourceType string, resourceID string) {
	cache.InvalidateSpecificResourceCache(optimizer, resourceType, resourceID)
}
