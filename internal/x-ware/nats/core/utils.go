package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/resources/common"
)

func (s *BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if m == nil || len(m.Data) == 0 || m.Data == nil {
		return fmt.Errorf("%s", errors.ERROR_INVALID_MESSAGE)
	}
	return nil
}

func (s *BaseSubscriber) AcknowledgeMessage(ctx context.Context, m *nats.Msg) error {
	return m.Ack()
}

func NewMessage(topic, name, scope string, resourceType common.Type, data interface{}) *Message {
	return &Message{
		Topic:        topic,
		ResourceName: name,
		ResourceType: resourceType,
		Scope:        scope,
		Data:         data,
	}
}

func GenerateKey(resourceName, resourceType, scope string) string {
	key := fmt.Sprintf("%s-%s-%s", resourceName, resourceType, scope)
	timestamp := time.Now().Unix() / 10
	return fmt.Sprintf("%s-%d", key, timestamp)
}

func GenerateDataHash(data []byte, subject string) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%s-%s", subject, hex.EncodeToString(hash[:]))
}

func GetParsedMessageHeader(m *nats.Msg) string {
	return m.Header.Get(KEY_PARSED_MESSAGE)
}

func SetParsedMessageHeader(m *nats.Msg, prefix, value string) {
	if prefix != "" {
		key := prefix + "_data"
		m.Header.Set(key, value)
	} else {
		m.Header.Set(KEY_PARSED_MESSAGE, value)
	}
}

func GenerateMessageId(subject string, data []byte) string {
	content := fmt.Sprintf("%s-%s-%d", subject, string(data), time.Now().UnixNano())
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16])
}
