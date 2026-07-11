package coordination

import (
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
)

var lg = constants.GetLogger(constants.LoggerPrefixDiscoveryManager)

func GenerateMsgPayload(appName, ns, cycleID, operation, enqueuedAt string, attempts int) map[string]any {
	return map[string]any{
		constants.StreamMsgFieldAppName:    appName,
		constants.StreamMsgFieldNamespace:  ns,
		constants.StreamMsgFieldCycleID:    cycleID,
		constants.StreamMsgFieldOperation:  operation,
		constants.StreamMsgFieldEnqueuedAt: enqueuedAt,
		constants.StreamMsgFieldAttempts:   strconv.Itoa(attempts),
	}
}

func MsgField(msg redis.XMessage, key string) string {
	if v, ok := msg.Values[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return constants.EmptyString
}
