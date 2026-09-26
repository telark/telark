package notifications

import "github.com/telark/rest/base"

const (
	Emit        base.Endpoint = "internal/notifications"
	List        base.Endpoint = "notifications"
	MarkRead    base.Endpoint = "notifications/{id}/read"
	MarkAllRead base.Endpoint = "notifications/read"
	Clear       base.Endpoint = "notifications"
)
