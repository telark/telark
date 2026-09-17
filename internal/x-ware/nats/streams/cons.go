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
	// Must be >= fetch batch + workers*queue of the largest consumer (notifier: 64 + 8*32); below it the server stops delivering.
	StreamMaxAckPending = 2048
	StreamAckWait       = 90 * time.Second
)
