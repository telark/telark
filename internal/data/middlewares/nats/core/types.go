package core

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/resources/common"
)

type NATSClient struct {
	Conn      *nats.Conn
	JetStream nats.JetStreamContext
}

type NATSConfig struct {
	User     string
	Password string
	Port     Port
}

// MessageHandler defines the function type for handling NATS messages
type MessageHandler func(*nats.Msg) error

// Message represents the common structure for all NATS messages
type Message struct {
	Topic        string      `json:"topic"`
	ResourceName string      `json:"resourceName"`
	ResourceType common.Type `json:"resourceType"`
	Scope        string      `json:"scope"`
	Data         interface{} `json:"data"`
}

// BaseSubscriber provides common functionality for NATS subscribers
type BaseSubscriber struct {
	Group          Group
	MaxRetries     int
	RetryDelay     time.Duration
	ProcessTimeout time.Duration
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
	APP_WORKLOADS   Group = "workloads_apps"
	BATCH_WORKLOADS Group = "workloads_batches"
	BRIDGES         Group = "bridges"
)

const (
	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"
)

const (
	maxRetries = 5
	retryDelay = 5 * time.Second
)
