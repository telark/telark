package cache

import (
	"fmt"

	"github.com/telark/exporter/internal/constants"
	rediscache "github.com/telark/x-ware/redis/cache"
)

var lg = constants.GetLogger(constants.PrefixCache)

// A write rarely knows which subjects its lists were cached under — a role edit
// reaches every user holding it — so the generation moves instead of the keys
// being hunted down, and every key derived from it stops being read at once.
type ListInvalidator interface {
	BumpListGeneration(resourceType string)
}

type ResourceInvalidator interface {
	ListInvalidator
	Delete(key string)
}

func InvalidateListCache(optimizer ListInvalidator, resourceType string) {
	optimizer.BumpListGeneration(resourceType)
}

func InvalidateGetCache(optimizer interface{ Delete(string) }, resourceType string, name string) {
	key := rediscache.GenerateKey(constants.OpGet, resourceType, name)
	optimizer.Delete(key)
}

func SmartInvalidateListCache(optimizer ListInvalidator, resourceType string, operation string) {
	modifyingOperations := []string{
		constants.OpCreate, constants.OpUpdate, constants.OpPatch, constants.OpDelete, constants.OpSync,
	}
	if rediscache.IsModifyingAction(operation, modifyingOperations) {
		InvalidateListCache(optimizer, resourceType)
	}
}

func InvalidateAllResourceCaches(optimizer ListInvalidator, resourceType string) {
	optimizer.BumpListGeneration(resourceType)
}

func InvalidateSpecificResourceCache(optimizer ResourceInvalidator, resourceType string, resourceName string) {
	optimizer.BumpListGeneration(resourceType)
	getKey := rediscache.GenerateKey(constants.OpGet, resourceType, resourceName)
	optimizer.Delete(getKey)
	lg.Info(fmt.Sprintf(string(constants.InfCacheInvalidatedSpecific), resourceType, resourceName))
}
