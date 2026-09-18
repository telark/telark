package publisher

import (
	"fmt"
	"runtime/debug"

	"github.com/telark/data/errors"
	resourceshared "github.com/telark/data/resources/shared"
	"github.com/telark/discovery/internal/constants"
	natscore "github.com/telark/x-ware/nats/core"
)

func PublishUpdate(
	params PublishUpdateParams,
	natsClient *natscore.NATSClient,
) error {
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrInPublishUpdate), r, debug.Stack()))
		}
	}()

	topic := natscore.GetTopicName(params.Group, natscore.Update)
	data := getDataBody(params.Scope, params.Data)
	msg := natscore.NewMessage(topic, params.Name, string(params.Scope), params.ResType, data)

	ack, err := PublishEvent(natsClient, topic, msg)
	if err != nil || ack == nil {
		lg.Error(fmt.Sprintf(string(errors.ErrNatsTopicPublish), topic, err))
		return fmt.Errorf(string(errors.ErrNatsTopicPublish), topic, err)
	}
	return nil
}

func getDataBody(scope resourceshared.DataScope, data any) any {
	switch scope {
	case resourceshared.ApplicationSpecScope:
		return data
	default:
		return nil
	}
}
