package init

import "time"

type RetryConfig struct {
	RetryInterval time.Duration
	MaxWait       time.Duration
}

