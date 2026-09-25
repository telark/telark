package cache

// Cache log/error messages.
const (
	MsgRedisClientNil         = "insights cache: Redis client is nil, insights will be empty"
	MsgCacheGetFailed         = "insights cache GET failed: key=%s err=%v"
	MsgCacheDeserializeFailed = "cache deserialize failed: key=%s err=%v raw=%s"
	MsgLegacyDocument         = "legacy document shape"
)
