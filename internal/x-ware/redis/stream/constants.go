package stream

import (
	"time"

	"github.com/plsyro/data/errors"
)

const (
	StreamConsumerGroupDeliveryID              = ">"
	StreamDefaultBlockDuration                 = 2 * time.Second
	StreamDefaultReadCount                     = 10
	StreamStaleClaimMinIdle                    = 5 * time.Minute
	StreamStaleClaimMaxCount                   = 10
	StreamStaleClaimInterval                   = 60 * time.Second
	LockDefaultTTL                             = 120 * time.Second
	LockHeartbeatInterval                      = 30 * time.Second
	LockReleaseMaxRetries                      = 3
	LockReleaseRetryDelay                      = 10 * time.Millisecond
	OperationStateTTL                          = 300 * time.Second
	DedupTTL                                   = 60 * time.Second
	ReplicaHeartbeatTTL                        = 30 * time.Second
	ReplicaHeartbeatInterval                   = 10 * time.Second
	ElectionDefaultTTL                         = 15 * time.Second
	ElectionRenewInterval                      = 5 * time.Second
	LockReleaseFailedMessage      errors.Error = "lock release failed after %d attempts: %s"
)
