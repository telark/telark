package core

import (
	"github.com/telark/data/errors"
)

const (
	EnvNatsHost     = "NATS_HOST"
	EnvNatsUser     = "NATS_USER"
	EnvNatsPassword = "NATS_PASSWORD"
)

const (
	ErrNatsHostRequired       errors.Error = "NATS_HOST env variable is required"
	ErrNatsUserRequired       errors.Error = "NATS_USER env variable is required"
	ErrNatsPasswordRequired   errors.Error = "NATS_PASSWORD env variable is required"
	ErrNatsClientNotConnected errors.Error = "NATS client is not connected"
	ErrFailedInitNatsClient   errors.Error = "failed to initialize NATS " +
		"client: %w"
)

const (
	DefaultConnectionTimeout = 10
	MaxReconnectAttempts     = 5
	ReconnectDelay           = 5
)

const (
	Client          port = 4222
	Monitoring      port = 8222

	Grouper        Group = "groupers"
	Applications   Group = "applications"
	AppWorkloads   Group = "workloads_apps"
	BatchWorkloads Group = "workloads_batches"
	Bridges        Group = "bridges"

	Create Action = "create"
	Update Action = "update"
	Delete Action = "delete"

	PrefixAck        = "$JS.ACK."
	KeyParsedMessage = "parsed_message"
)
