package user

import (
	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	"github.com/telark/exporter/utils/performance"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
)

func InvalidateUserCaches(optimizer *performance.Optimizer, userID string) {
	username, err := GetUsernameFromUser(userID)
	if err == nil && username != constants.EmptyString {
		cache.InvalidateGetCache(optimizer, constants.ResourceUser, username)
	}

	resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceUser, userID)
}
