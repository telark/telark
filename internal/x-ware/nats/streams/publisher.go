package streams

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/x-ware/nats/core"
)

func PublishMessage(c *core.NATSClient, subj string, data []byte) (
	*nats.PubAck,
	error,
) {
	if c == nil || c.JetStream == nil {
		return nil, fmt.Errorf("%s", errors.ErrNatsJetstreamNotInitialized)
	}

	msgID := core.GenerateMessageID(subj, data)
	ack, err := c.JetStream.Publish(subj, data, nats.MsgId(msgID))
	return ack, err
}
