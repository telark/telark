package streams

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/middlewares/nats/core"
)

func PublishMessage(c *core.NATSClient, subj string, data []byte) (*nats.PubAck, error) {
	if c == nil || c.JetStream == nil {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_JETSTREAM_NOT_INITIALIZED))
	}

	// Generate unique message ID for deduplication
	msgId := generateMessageId(subj, data)

	ack, err := c.JetStream.Publish(subj, data, nats.MsgId(msgId))
	return ack, err
}

func PublishMessageWithId(c *core.NATSClient, subj string, data []byte, msgId string) (*nats.PubAck, error) {
	if c == nil || c.JetStream == nil {
		return nil, fmt.Errorf(string(errors.ERROR_NATS_JETSTREAM_NOT_INITIALIZED))
	}

	// Use provided message ID or generate one if empty
	if msgId == "" {
		msgId = generateMessageId(subj, data)
	}

	ack, err := c.JetStream.Publish(subj, data, nats.MsgId(msgId))
	return ack, err
}

func generateMessageId(subject string, data []byte) string {
	content := fmt.Sprintf("%s-%s-%d", subject, string(data), time.Now().UnixNano())
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16])
}
