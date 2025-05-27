package nats

import "github.com/nats-io/nats.go"

func (c *NATSClient) PublishMessage(topic string, data []byte) error {
	_, err := c.JetStream.Publish(topic, data)
	return err
}

func (c *NATSClient) SubscribeToTopicWithQueue(topic, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.QueueSubscribe(topic, queue, handler, nats.DeliverAll())
}

func (c *NATSClient) SubscribeToTopic(topic string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.Subscribe(topic, handler, nats.DeliverAll())
}

func (c *NATSClient) PullSubscribe(topic string) (*nats.Subscription, error) {
	return c.JetStream.PullSubscribe(topic, "pull-sub", nats.DeliverAll())
}
