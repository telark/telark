package constants

import "github.com/plsyro/data/errors"

const (
	ErrNatsInitRetrying        errors.Error = "[nats] NATS not ready, retrying in %ds (elapsed: %ds)"
	ErrNatsInitMaxWaitExceeded errors.Error = "[nats] NATS did not become ready within %ds"
)
