package constants

import "github.com/telark/data/messages"

const (
	InfoSubscribersStarted messages.Message = "subscribers started with success"
	InfoStatusReady        messages.Message = "notifier connected"
	InfoStatusNotReady     messages.Message = "notifier not connected to NATS"
	InfoNatsStartRetrying  messages.Message = "nats start failed, retrying in %ds"
)

const (
	ErrStatusServerShutdown = "status server shutdown failed: %v"
	ErrNatsFetchMessages    = "failed to fetch messages on topic %s: %v"
)
