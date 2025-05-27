package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/resources/common"
)

// MessageHandler defines the function type for handling NATS messages
type MessageHandler func(*nats.Msg) error

type NATSConfig struct {
	User     string
	Password string
	Port     Port
}

// Message represents the common structure for all NATS messages
type Message struct {
	Topic        string      `json:"topic"`
	ResourceName string      `json:"resourceName"`
	ResourceType common.Type `json:"resourceType"`
	Scope        string      `json:"string"`
	Data         interface{} `json:"data"`
}

// BaseSubscriber provides common functionality for NATS subscribers
type BaseSubscriber struct {
	ResourceType   common.Type
	Group          Group
	MaxRetries     int
	RetryDelay     time.Duration
	ProcessTimeout time.Duration
}

// StreamManager handles JetStream stream operations
type StreamManager struct {
	js nats.JetStreamContext
}

type (
	Port   int
	Group  string
	Action string
)

const (
	CLIENT     Port = 4222
	MONITORING Port = 8222
)

const (
	GROUPER         Group = "groupers"
	APP_WORKLOADS   Group = "workloads.apps"
	BATCH_WORKLOADS Group = "workloads.batches"
	BRIDGES         Group = "bridges"
)

const (
	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"
)

// GetTopicName generates a topic string for a given group and action
func GetTopicName(group Group, action Action) string {
	return fmt.Sprintf("%s.%s.%s", global.BaseNamespace, group, action)
}

// GetQueueName generates a queue name for a given group and action
func GetQueueName(group Group, action Action) string {
	return fmt.Sprintf("%s-%s-%s-queue", global.BaseNamespace, group, action)
}

// GetGroup returns the subscriber's group
func (s *BaseSubscriber) GetGroup() Group {
	return s.Group
}
