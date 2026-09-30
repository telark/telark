package performance

import (
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
)

func GetTimeoutForResource(resourceType, operation string) time.Duration {
	if timeouts, exists := DefaultTimeoutConfig.ResourceTimeouts[resourceType]; exists {
		if timeout, exists := timeouts[operation]; exists {
			return timeout
		}
	}

	switch operation {
	case constants.OpCreate:
		return DefaultTimeoutConfig.CreateTimeout
	case constants.OpList:
		return DefaultTimeoutConfig.ListTimeout
	case constants.OpUpdate:
		return DefaultTimeoutConfig.UpdateTimeout
	case constants.OpPatch:
		return DefaultTimeoutConfig.PatchTimeout
	case constants.OpDelete:
		return DefaultTimeoutConfig.DeleteTimeout
	default:
		return DefaultTimeoutConfig.GetTimeout
	}
}
