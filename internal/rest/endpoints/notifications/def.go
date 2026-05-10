package notifications

import "github.com/plsyro/rest/base"

const (
	Emit        base.Endpoint = "notifications/emit"
	List        base.Endpoint = "notifications/get"
	MarkRead    base.Endpoint = "notifications/{id}/markasread"
	MarkAllRead base.Endpoint = "notifications/markallread"
	Clear       base.Endpoint = "notifications/clear"
)
