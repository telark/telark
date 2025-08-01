package core

import (
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/resources/common"
)

type (
	MessageHandler func(*nats.Msg) error
	Port           int
	Group          string
	Action         string
	NATSClient     struct {
		Conn      *nats.Conn
		JetStream nats.JetStreamContext
	}
	NATSConfig struct {
		User     string
		Password string
		Port     Port
	}
	Message struct {
		Topic        string      `json:"topic"`
		ResourceName string      `json:"resourceName"`
		ResourceType common.Type `json:"resourceType"`
		Scope        string      `json:"scope"`
		Data         interface{} `json:"data"`
	}
	BaseSubscriber struct {
		Group          Group
		MaxRetries     int
		RetryDelay     time.Duration
		ProcessTimeout time.Duration
	}
)

const (
	// Self
	CLIENT            Port = 4222
	MONITORING        Port = 8222
	NATS_SERVICE_NAME      = "nats-service"

	// Resource Types
	GROUPER         Group = "groupers"
	APP_WORKLOADS   Group = "workloads_apps"
	BATCH_WORKLOADS Group = "workloads_batches"
	BRIDGES         Group = "bridges"

	// Actions
	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"

	// Config
	maxRetries = 5
	retryDelay = 5 * time.Second

	// Prefixes & Keys
	PREFIX_ACK         = "$JS.ACK."
	KEY_PARSED_MESSAGE = "parsed_message"
)
