package applications

import (
	"github.com/nats-io/nats.go"
	natscore "github.com/telark/x-ware/nats/core"
)

func (s *ApplicationSubscriber) getHandler(action natscore.Action) func(*nats.Msg) error {
	switch action {
	case natscore.Update:
		return s.handleUpdate
	case natscore.Delete:
		return s.handleDelete
	default:
		return s.HandleUnknown
	}
}
