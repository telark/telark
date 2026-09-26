package category

import (
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
)

func InvalidateCategoryCaches(optimizer *performance.Optimizer) {
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceCategory)

	categories, err := GetAllCategories()
	if err != nil {
		return
	}

	scopes := make(map[string]bool)
	for _, cat := range categories {
		if scope, ok := cat[constants.FieldScope].(string); ok && scope != constants.EmptyString {
			scopes[scope] = true
		}
	}

	for scope := range scopes {
		cache.InvalidateGetCache(optimizer, constants.ResourceCategory, scope)
	}
}
