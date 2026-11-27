package constants

import "github.com/telark/telark/internal/data/messages"

const (
	InfoStatusReady             messages.Message = "notifier connected"
	InfoStatusNotReady          messages.Message = "notifier not connected to NATS"
	InfoNatsStartRetrying       messages.Message = "nats start failed, retrying in %ds"
	WarnApplyDrainTimeout       messages.Message = "apply workers did not drain in time, unacked messages will be redelivered"
	InfoSkippingStaleRedelivery messages.Message = "skipping stale redelivery for %s: stream seq %d behind applied %d"
	WarnNatsConsumerRestored    messages.Message = "the NATS server had no stream or consumer for %s, recreated them"
)

const (
	ErrStatusServerShutdown = "status server shutdown failed: %v"
	ErrNatsFetchMessages    = "failed to fetch messages on topic %s: %v"
	ErrNatsRestoreConsumer  = "failed to recreate the stream and consumer for %s: %v"
	ErrHandlerPanicked      = "handler panicked on topic %s: %v"
)
