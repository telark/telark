package user

import (
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
)

func InvalidateUserCaches(optimizer *performance.Optimizer, userID string) {
	username, err := GetUsernameFromUser(userID)
	if err == nil && username != constants.EmptyString {
		cache.InvalidateGetCache(optimizer, constants.ResourceUser, username)
	}

	resourcesshared.InvalidateResourceCaches(optimizer, constants.ResourceUser, userID)
}
