package streams

import (
	"github.com/plsyro/data-pkg/middlewares/nats/core"
)

func PublishMessage(c *core.NATSClient, topic string, data []byte) error {
	_, err := c.JetStream.Publish(topic, data)
	return err
}
