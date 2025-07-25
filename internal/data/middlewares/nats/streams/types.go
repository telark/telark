package streams

import (
	"time"

	"github.com/nats-io/nats.go"
)

const (
	STREAM_STORAGE_TYPE      = nats.FileStorage
	STREAM_RETENTION_POLICY  = nats.WorkQueuePolicy
	STREAM_MAX_AGE_RETENTION = 24 * time.Hour
	STREAM_ALLOW_ROLLUP      = false
	STREAM_ALLOW_DIRECT      = false
	STREAM_MAX_DELIVER_COUNT = 3
	STREAM_PULL_SUB_DURABLE  = "pull-sub"
)
