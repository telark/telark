package retry

import (
	"context"
	"strings"
	"time"

	"github.com/plsyro/kcore/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Backoff describes a single bounded exponential backoff schedule.
type Backoff struct {
	Steps    int
	Duration time.Duration
	Factor   float64
	Jitter   float64
	Cap      time.Duration
}

// DefaultTransient returns the backoff used for transient K8s GET retries
// (snapshot fanout, manifest fetch).
func DefaultTransient() Backoff {
	return Backoff{
		Steps:    constants.K8sTransientRetrySteps,
		Duration: constants.K8sTransientRetryInitial,
		Factor:   constants.K8sTransientRetryFactor,
		Jitter:   constants.K8sTransientRetryJitter,
		Cap:      constants.K8sTransientRetryCap,
	}
}

// DefaultApply returns the backoff used for SSA Apply retries.
func DefaultApply() Backoff {
	return Backoff{
		Steps:    constants.K8sApplyRetrySteps,
		Duration: constants.K8sApplyRetryInitial,
		Factor:   constants.K8sApplyRetryFactor,
		Jitter:   constants.K8sApplyRetryJitter,
		Cap:      constants.K8sApplyRetryCap,
	}
}

// IsTransientK8sError reports whether err should be retried by callers
// hitting the apiserver. Covers client-side throttling (rate limiter Wait),
// 429, 503, request timeout, and apiserver service unavailability.
func IsTransientK8sError(err error) bool {
	if err == nil {
		return false
	}
	if k8serrors.IsTooManyRequests(err) ||
		k8serrors.IsServerTimeout(err) ||
		k8serrors.IsServiceUnavailable(err) ||
		k8serrors.IsTimeout(err) {
		return true
	}
	if strings.Contains(err.Error(), constants.K8sClientRateLimitWaitSub) {
		return true
	}
	return false
}

// OnTransient retries fn while IsTransientK8sError(err) is true, up to b.Steps
// attempts using exponential backoff. The final error (or success) is returned.
// ctx cancellation aborts the wait early; the in-flight fn call is the caller's
// responsibility to bind to ctx.
func OnTransient(ctx context.Context, b Backoff, fn func() error) error {
	wb := wait.Backoff{
		Steps:    b.Steps,
		Duration: b.Duration,
		Factor:   b.Factor,
		Jitter:   b.Jitter,
		Cap:      b.Cap,
	}
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, wb, func(context.Context) (bool, error) {
		lastErr = fn()
		if lastErr == nil {
			return true, nil
		}
		if !IsTransientK8sError(lastErr) {
			return false, lastErr
		}
		return false, nil
	})
	if err == nil {
		return nil
	}
	if wait.Interrupted(err) && lastErr != nil {
		return lastErr
	}
	return err
}
