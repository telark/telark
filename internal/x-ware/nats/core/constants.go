package core

import (
	"github.com/plsyro/data-pkg/errors"
)

const (
	EnvNatsUser     = "NATS_USER"
	EnvNatsPassword = "NATS_PASSWORD"
)

const (
	ErrNatsUserRequired       errors.Error = "NATS_USER environment variable is required"
	ErrNatsPasswordRequired   errors.Error = "NATS_PASSWORD environment variable is required"
	ErrNatsClientNotConnected errors.Error = "NATS client is not connected"
	ErrFailedInitNatsClient   errors.Error = "failed to initialize NATS client: %w"
)

const (
	DefaultConnectionTimeout = 10
	MaxReconnectAttempts     = 5
	ReconnectDelay           = 5
)

const (
	Client          Port = 4222
	Monitoring      Port = 8222
	NatsServiceName      = "nats-service"

	Grouper        Group = "groupers"
	AppWorkloads   Group = "workloads_apps"
	BatchWorkloads Group = "workloads_batches"
	Bridges        Group = "bridges"

	Create Action = "create"
	Update Action = "update"
	Delete Action = "delete"

	PrefixAck        = "$JS.ACK."
	KeyParsedMessage = "parsed_message"
)
