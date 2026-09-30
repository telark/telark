package logdedup

import (
	"fmt"
	"time"

	"github.com/telark/telark/services/discovery/internal/constants"
)

const DefaultWindow = 30 * time.Second

func New(window time.Duration) *Dedupe {
	if window <= constants.ZeroDuration {
		window = DefaultWindow
	}
	return &Dedupe{buckets: map[string]bucket{}, window: window}
}

func (d *Dedupe) ErrorOnce(scope, format string, err error) {
	if !d.shouldLog(scope, signature(err)) {
		return
	}
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).
		Error(fmt.Sprintf(format, err))
}

func (d *Dedupe) Reset(scope string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.buckets, scope)
}

func (d *Dedupe) shouldLog(scope, sig string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	b, ok := d.buckets[scope]
	if ok && b.signature == sig && now.Sub(b.loggedAt) < d.window {
		return false
	}
	d.buckets[scope] = bucket{signature: sig, loggedAt: now}
	return true
}

func signature(err error) string {
	if err == nil {
		return constants.EmptyString
	}
	return err.Error()
}
