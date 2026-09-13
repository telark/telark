package performance

import (
	"time"

	"github.com/telark/exporter/internal/constants"
)

func GetTimeoutForResource(resourceType, operation string) time.Duration {
	if timeouts, exists := DefaultTimeoutConfig.ResourceTimeouts[resourceType]; exists {
		if timeout, exists := timeouts[operation]; exists {
			return timeout
		}
	}

	fallback := map[string]time.Duration{
		string(constants.OpCreate): DefaultTimeoutConfig.CreateTimeout,
		string(constants.OpGet):    DefaultTimeoutConfig.GetTimeout,
		string(constants.OpList):   DefaultTimeoutConfig.ListTimeout,
		string(constants.OpUpdate): DefaultTimeoutConfig.UpdateTimeout,
		string(constants.OpPatch):  DefaultTimeoutConfig.PatchTimeout,
		string(constants.OpDelete): DefaultTimeoutConfig.DeleteTimeout,
	}

	if t, ok := fallback[operation]; ok {
		return t
	}
	return DefaultTimeoutConfig.GetTimeout
}
