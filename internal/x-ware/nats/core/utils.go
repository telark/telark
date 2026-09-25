package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	resourceshared "github.com/telark/data/resources/shared"
	globalshared "github.com/telark/data/shared"
)

const (
	emptyDataLength  = 0
	timestampDivisor = 10
	hashLength       = 16
)

func (*BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if m == nil || len(m.Data) == emptyDataLength {
		return fmt.Errorf("%s", errors.ErrNatsEmptyMsgData)
	}
	return nil
}

func (*BaseSubscriber) AcknowledgeMessage(m *nats.Msg) error {
	return m.Ack()
}

func NewMessage(topic, name, scope string, resourceType resourceshared.Type,
	data any,
) *Message {
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
	timestamp := time.Now().Unix() / timestampDivisor
	return fmt.Sprintf("%s-%d", key, timestamp)
}

func GenerateDataHash(data []byte, subject string) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%s-%s", subject, hex.EncodeToString(hash[:]))
}

func GetParsedMessageHeader(m *nats.Msg) string {
	return m.Header.Get(KeyParsedMessage)
}

func SetParsedMessageHeader(m *nats.Msg, prefix, value string) {
	if prefix != "" {
		key := prefix + "_data"
		m.Header.Set(key, value)
	} else {
		m.Header.Set(KeyParsedMessage, value)
	}
}

func GenerateMessageID(subject string, data []byte) string {
	content := fmt.Sprintf("%s-%s-%d", subject, string(data),
		time.Now().UnixNano())
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:hashLength])
}

func GenerateSubjectName(group Group) string {
	return fmt.Sprintf("%s.%s.*", globalshared.BaseNamespace, group)
}
