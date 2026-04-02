package stream

import "time"

const (
	StreamConsumerGroupDeliveryID = ">"
	StreamDefaultBlockDuration    = 2 * time.Second
	StreamDefaultReadCount        = 10
	StreamStaleClaimMinIdle       = 5 * time.Minute
	StreamStaleClaimMaxCount      = 10
	StreamStaleClaimInterval      = 60 * time.Second

	LockDefaultTTL        = 120 * time.Second
	LockHeartbeatInterval = 30 * time.Second

	OperationStateTTL = 300 * time.Second
	DedupTTL          = 60 * time.Second

	ReplicaHeartbeatTTL      = 30 * time.Second
	ReplicaHeartbeatInterval = 10 * time.Second

	ElectionDefaultTTL    = 60 * time.Second
	ElectionRenewInterval = 20 * time.Second
)
