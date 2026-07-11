package applications

import (
	"strings"

	"github.com/nats-io/nats.go"
	resourceshared "github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/subscribers/base"
	natscore "github.com/telark/x-ware/nats/core"
)

type ApplicationSubscriber struct {
	*base.BaseSubscriber
}

func NewApplicationSubscriber() natscore.ResourceSubscriber {
	subscriber := &ApplicationSubscriber{
		BaseSubscriber: base.NewBaseSubscriber(natscore.Applications, resourceshared.Application, nil),
	}

	subscriber.SetHandlerCallback(subscriber.executeHandler)
	return subscriber
}

func (s *ApplicationSubscriber) executeHandler(m *nats.Msg, action natscore.Action) error {
	return base.SharedExecuteHandler(
		m,
		action,
		strings.ToLower(string(resourceshared.Application)),
		s.getHandler,
		nil,
	)
}
