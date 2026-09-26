package streams

import (
	"errors"

	"github.com/nats-io/nats.go"
	"github.com/telark/x-ware/nats/core"
)

func DefaultConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    StreamMaxDeliverCount,
		MaxAckPending: StreamMaxAckPending,
		AckWait:       StreamAckWait,
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
	info, err := c.JetStream.AddConsumer(streamName, cfg)
	// A durable left by an older release keeps its AckWait/MaxAckPending, and
	// AddConsumer rejects the mismatch instead of converging it.
	if errors.Is(err, nats.ErrConsumerNameAlreadyInUse) {
		return c.JetStream.UpdateConsumer(streamName, cfg)
	}
	return info, err
}
