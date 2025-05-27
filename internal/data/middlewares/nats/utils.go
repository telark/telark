package nats

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
)

// ValidateMessage validates a NATS message
func (s *BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if m == nil || len(m.Data) == 0 {
		return fmt.Errorf("%s", errors.ERROR_INVALID_MESSAGE)
	}
	return nil
}

// ProcessMessage processes a NATS message
func (s *BaseSubscriber) ProcessMessage(ctx context.Context, m *nats.Msg) error {
	return m.Ack()
}

// NATSClient methods for message operations
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
