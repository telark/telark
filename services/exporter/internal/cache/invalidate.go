package cache

import (
	"fmt"

	"github.com/telark/exporter/constants"
	rediscache "github.com/telark/x-ware/redis/cache"
)

var lg = constants.GetLogger(constants.PrefixCache)

func InvalidateListCache(optimizer interface{ Delete(string) }, resourceType string) {
	key := rediscache.GenerateKey(string(constants.OpList), resourceType, constants.EmptyString)
	optimizer.Delete(key)
}

func InvalidateGetCache(optimizer interface{ Delete(string) }, resourceType string, name string) {
	key := rediscache.GenerateKey(string(constants.OpGet), resourceType, name)
	optimizer.Delete(key)
}

func SmartInvalidateListCache(optimizer interface{ Delete(string) }, resourceType string, operation string) {
	modifyingOperations := []string{
		constants.OpCreate, constants.OpUpdate, constants.OpPatch, constants.OpDelete, constants.OpSync,
	}
	if rediscache.IsModifyingAction(operation, modifyingOperations) {
		InvalidateListCache(optimizer, resourceType)
	}
}

func InvalidateAllResourceCaches(optimizer interface{ Delete(string) }, resourceType string) {
	listKey := rediscache.GenerateKey(string(constants.OpList), resourceType, constants.EmptyString)
	optimizer.Delete(listKey)
	getKeyPattern := rediscache.GenerateKey(string(constants.OpGet), resourceType, "*")
	lg.Info(fmt.Sprintf(string(constants.InfCacheInvalidatingPattern), getKeyPattern))
	lg.Info(fmt.Sprintf(string(constants.InfCacheInvalidateResourceType), resourceType))
}

func InvalidateSpecificResourceCache(optimizer interface{ Delete(string) }, resourceType string, resourceName string) {
	listKey := rediscache.GenerateKey(string(constants.OpList), resourceType, constants.EmptyString)
	optimizer.Delete(listKey)
	getKey := rediscache.GenerateKey(string(constants.OpGet), resourceType, resourceName)
	optimizer.Delete(getKey)
	versionedKeyPattern := getKey + ":*"
	lg.Info(fmt.Sprintf(string(constants.InfCacheInvalidatingVersionedKeys), versionedKeyPattern))
	lg.Info(fmt.Sprintf(string(constants.InfCacheInvalidatedSpecific), resourceType, resourceName))
}
