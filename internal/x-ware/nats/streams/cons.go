package streams

import (
	"time"

	"github.com/nats-io/nats.go"
)

const (
	StreamStorageType     = nats.FileStorage
	StreamRetentionPolicy = nats.WorkQueuePolicy
	StreamMaxAgeRetention = 24 * time.Hour
	StreamAllowRollup     = false
	StreamAllowDirect     = false
	StreamMaxDeliverCount = 3
	StreamPullSubDurable  = "pull-sub"
)
