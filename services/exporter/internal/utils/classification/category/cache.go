package category

import (
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
)

// Scoped lists (?scope=) share the list generation, so one bump drops them all.
func InvalidateCategoryCaches(optimizer *performance.Optimizer) {
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceCategory)
}
