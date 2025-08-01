package streams

import (
	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/middlewares/nats/core"
)

func SubscribeToTopicWithQueue(c *core.NATSClient, topic, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.QueueSubscribe(topic, queue, handler, nats.DeliverAll())
}

func SubscribeToTopic(c *core.NATSClient, topic string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.Subscribe(topic, handler, nats.DeliverAll())
}

func PullSubscribe(c *core.NATSClient, topic string) (*nats.Subscription, error) {
	return c.JetStream.PullSubscribe(topic, STREAM_PULL_SUB_DURABLE, nats.DeliverAll())
}
