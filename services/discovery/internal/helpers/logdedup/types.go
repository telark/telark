package logdedup

import (
	"sync"
	"time"
)

type bucket struct {
	signature string
	loggedAt  time.Time
}

type Dedupe struct {
	mu      sync.Mutex
	buckets map[string]bucket
	window  time.Duration
}
