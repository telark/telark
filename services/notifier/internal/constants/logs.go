package constants

import "github.com/telark/data/messages"

const (
	InfoSubscribersStarted messages.Message = "subscribers started with success"
	InfoStatusReady        messages.Message = "notifier connected"
	InfoStatusNotReady     messages.Message = "notifier not connected to NATS"
)

const ErrStatusServerShutdown = "status server shutdown failed: %v"
