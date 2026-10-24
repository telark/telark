package notifications

import "github.com/telark/telark/internal/rest/base"

const (
	Emit        base.Endpoint = "internal/notifications"
	List        base.Endpoint = "notifications"
	Events      base.Endpoint = "notifications/events"
	MarkRead    base.Endpoint = "notifications/{id}/read"
	MarkAllRead base.Endpoint = "notifications/read"
	Clear       base.Endpoint = "notifications"
	Delete      base.Endpoint = "notifications/{id}"
)
