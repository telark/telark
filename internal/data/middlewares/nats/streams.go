package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
)

type Action string

const (
	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"
)

type Scope string

const (
	NAMESPACE  Scope = "namespaces"
	DEPLOYMENT Scope = "deployments"
)

func GetTopic(scope Scope, action Action) string {
	return fmt.Sprintf("%s.%s.%s", global.BaseNamespace, scope, action)
}

func (c *NATSClient) PublishMessage(topic string, data []byte) error {
	_, err := c.JetStream.Publish(topic, data)
	return err
}

func (c *NATSClient) SubscribeToTopic(topic string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.Subscribe(topic, handler, nats.DeliverNew())
}

func (c *NATSClient) SubscribeToTopicWithQueue(topic, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return c.JetStream.QueueSubscribe(topic, queue, handler, nats.DeliverNew())
}

func (c *NATSClient) PullSubscribe(topic string) (*nats.Subscription, error) {
	return c.JetStream.PullSubscribe(topic, "pull-sub", nats.DeliverNew())
}

func (c *NATSClient) GetStreamInfo(streamName string) (*nats.StreamInfo, error) {
	return c.JetStream.StreamInfo(streamName)
}
