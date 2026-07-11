package streams

import (
	"github.com/nats-io/nats.go"
	"github.com/telark/x-ware/nats/core"
)

func DefaultConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    StreamMaxDeliverCount,
	}
}

func CreateConsumer(c *core.NATSClient, streamName, consumerName, topic,
	queue string,
) (*nats.ConsumerInfo, error) {
	cfg := DefaultConsumerConfig()
	cfg.Name = consumerName
	cfg.Durable = consumerName
	cfg.FilterSubject = topic
	cfg.DeliverGroup = queue
	return c.JetStream.AddConsumer(streamName, cfg)
}
