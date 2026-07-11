package redis

type (
	RedisSuffix string
	RedisPrefix string
)

const (
	RedisKeySuffixPhase                 RedisSuffix = "phase"
	RedisKeySuffixRun                   RedisSuffix = "run"
	RedisKeySuffixLastRun               RedisSuffix = "last_run"
	RedisKeySuffixAppliedHash           RedisSuffix = "applied_hash"
	RedisKeySuffixRunStatus             RedisSuffix = "run_status"
	RedisKeySuffixExists                RedisSuffix = "exists"
	RedisKeySuffixFirstSeen             RedisSuffix = "first_seen"
	RedisKeySuffixLastSeen              RedisSuffix = "last_seen"
	RedisKeySuffixStatus                RedisSuffix = "status"
	RedisKeySuffixLock                  RedisSuffix = "lock"
	RedisKeySuffixLastSync              RedisSuffix = "last_sync"
	RedisKeySuffixProcessID             RedisSuffix = "process_id"
	RedisKeyPrefixSyncerDesired         RedisPrefix = "syncer:desired"
	RedisKeyPrefixSyncerStatus          RedisPrefix = "syncer:status"
	RedisKeyPrefixSyncerLastHeartbeat   RedisPrefix = "syncer:last_heartbeat"
	RedisKeyPrefixInformerDesired       RedisPrefix = "informer:desired"
	RedisKeyPrefixInformerStatus        RedisPrefix = "informer:status"
	RedisKeyPrefixInformerLastHeartbeat RedisPrefix = "informer:last_heartbeat"
	RedisKeyPrefixSync                  RedisPrefix = "sync"
	RedisKeyPrefixInformer              RedisPrefix = "informer"
	RedisKeySyncsSingle                             = "single"
)
