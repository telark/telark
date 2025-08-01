package core

import (
	"time"

	"github.com/plsyro/data-pkg/errors"
)

const (
	ENV_NATS_USER     = "NATS_USER"
	ENV_NATS_PASSWORD = "NATS_PASSWORD"
)

const (
	ERROR_NATS_USER_REQUIRED        errors.Error = "NATS_USER environment variable is required"
	ERROR_NATS_PASSWORD_REQUIRED    errors.Error = "NATS_PASSWORD environment variable is required"
	ERROR_NATS_CLIENT_NOT_CONNECTED errors.Error = "NATS client is not connected"
	ERROR_FAILED_INIT_NATS_CLIENT   errors.Error = "failed to initialize NATS client: %w"
)

const (
	DEFAULT_CONNECTION_TIMEOUT = 10
	MAX_RECONNECT_ATTEMPTS     = 5
	RECONNECT_DELAY            = 5
)

const (
	CLIENT            Port = 4222
	MONITORING        Port = 8222
	NATS_SERVICE_NAME      = "nats-service"

	GROUPER         Group = "groupers"
	APP_WORKLOADS   Group = "workloads_apps"
	BATCH_WORKLOADS Group = "workloads_batches"
	BRIDGES         Group = "bridges"

	CREATE Action = "create"
	UPDATE Action = "update"
	DELETE Action = "delete"

	maxRetries = 5
	retryDelay = 5 * time.Second

	PREFIX_ACK         = "$JS.ACK."
	KEY_PARSED_MESSAGE = "parsed_message"
)
