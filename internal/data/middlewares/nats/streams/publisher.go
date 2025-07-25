package streams

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/middlewares/nats/core"
)

func PublishMessage(c *core.NATSClient, subj string, data []byte) (*nats.PubAck, error) {
	if c == nil || c.JetStream == nil {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_JETSTREAM_NOT_INITIALIZED))
	}

	msgId := core.GenerateMessageId(subj, data)
	ack, err := c.JetStream.Publish(subj, data, nats.MsgId(msgId))
	return ack, err
}
