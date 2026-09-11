# TDD Evidence — notifier NATS connection never retries

**Date:** 2026-09-11
**Service:** `services/notifier`
**Source plan:** none — journeys derived during this TDD run from the reported defect.

## Reported defect

> "notifier nats connection failing as when it fails it does not retry until I restart manually the service and this pattern is fixed already in the exporter-service."

## Root cause

`notifier/main.go` called `Start()` exactly once:

```go
m := manager.NewManager()
if err := m.Start(); err != nil {
    lg.Error(...)   // logged, then execution continues — no retry, ever
}
```

`Start()` delegates to `natsinit.NewClientWithRetry`, which retries only until
`MaxWait` (`constants.NatsInitMaxWaitSeconds = 300`) and then returns `nil`
(`x-ware/nats/init/client.go`). After that window the client is nil, `Start()`
returns an error, `main` logs it, and the process keeps running with **no
subscribers** until an operator restarts it. `logConnectionStatus` then reports
"disconnected" on a ticker forever.

Reference pattern in exporter-service: `initOptimizerWithRetry`
(`services/exporter/main.go:133-144`) — an unbounded retry loop with a sleep.
Ported here, with a context guard added so shutdown still wins.

## User journeys

1. As an operator, when NATS is unavailable at notifier startup, I want notifier
   to keep retrying until it connects, so I never have to restart it manually.
2. As an operator, when I stop the service, I want an in-progress retry loop to
   exit promptly rather than block shutdown.
3. As an operator, when retries are happening, I want each failed attempt logged
   so a stuck notifier is distinguishable from a healthy one.

## Task report

**Execution:** extracted the retry loop as `manager.RetryStart` — a context-aware
function taking `start func() error`, so it is unit-testable in milliseconds
(the inner dial has a package-level client cache and 5-minute waits, making it
unsuitable as a direct test seam). `main.go` now runs it in a goroutine and sets
connectivity-ready only on success.

**RED** — `go test ./internal/tests/manager/...`

```
manager [build failed]
  internal/tests/manager/retry_test.go:42:20: undefined: manager.RetryStart
  internal/tests/manager/retry_test.go:54:20: undefined: manager.RetryStart
  internal/tests/manager/retry_test.go:70:30: undefined: manager.RetryStart
  internal/tests/manager/retry_test.go:91:20: undefined: manager.RetryStart
```

Compile-time RED: the new tests reference the missing production seam. The
failure is the intended missing implementation, not broken setup.

**GREEN** — `go test ./internal/tests/manager/... -run TestRetryStart`

```
Go test: 4 passed in 1 packages
```

Full suite after wiring `main.go`: `go test ./...` → `65 passed in 11 packages`.

## Test specification

| # | What is guaranteed | Test | Type | Result | Evidence |
|---|--------------------|------|------|--------|----------|
| 1 | A NATS outage heals without a restart — Start is retried until it connects | `internal/tests/manager/retry_test.go:TestRetryStartRetriesUntilSuccess` | unit | PASS | `go test ./internal/tests/manager/...` |
| 2 | A Start that succeeds first time is not retried | `retry_test.go:TestRetryStartStopsAfterFirstSuccess` | unit | PASS | same |
| 3 | A permanently failing Start stops on context cancellation instead of blocking shutdown | `retry_test.go:TestRetryStartStopsOnContextCancel` | unit | PASS | same |
| 4 | An already-cancelled context performs no connection attempt at all | `retry_test.go:TestRetryStartHonoursCancelledContext` | unit | PASS | same |
| 5 | Every failed attempt is reported to the operator (error + retry warning) | `retry_test.go:TestRetryStartLogsEachFailedAttempt` | unit | PASS | same |
| 6 | A non-positive interval falls back to the configured default, never a busy loop | `retry_test.go:TestRetryStartDefaultsNonPositiveInterval` | unit | PASS | same |

## Verification commands

| Command | Result |
|---------|--------|
| `go build ./...` | Success |
| `go test ./...` | 65 passed, 11 packages |
| `go test ./internal/tests/manager/... -race -count=2` | 14 passed, no races |
| `golangci-lint run` (project config, no overrides, no `//nolint`) | No issues found |
| `go test ./... -coverpkg=./...` | `RetryStart` **100.0%**; service total 77.4% |

## Files changed

- `internal/subscribers/manager/retry.go` — **new**, `RetryStart`
- `internal/constants/config.go` — added `NatsStartRetrySeconds = 15`
- `internal/constants/logs.go` — added `InfoNatsStartRetrying`
- `main.go` — one-shot `Start()` replaced with the retry goroutine; removed the
  `data/errors` import that this change orphaned
- `internal/tests/manager/retry_test.go` — **new**, 6 tests

## Coverage and known gaps

- The changed code is **100%** covered. Service-wide total is **77.4%**, below
  the 80% bar — this is the pre-existing baseline (76.6% before this change, so
  the change raised it). Closing the remaining gap means testing code unrelated
  to this defect; deliberately not done here.
- **Not fixed, flagged deliberately:** `manager.Start()` logs but does **not**
  return the error from `startSubscribers` (`base.go:73-75`) and then returns
  `nil`, so a connection that succeeds while subscribers fail is reported as
  success — `RetryStart` would stop and readiness would be set. The independent
  log audit corroborated this (`base.go:76` emits "subscribers started with
  success" even on that failure path). Fixing it changes behaviour beyond the
  reported defect: the service would retry instead of running degraded. Left for
  an explicit decision.
- Recovery time is bounded by the inner `MaxWait` (300s) plus the outer interval
  (15s), because each outer attempt still absorbs a full inner retry window.
  Acceptable, and left unchanged to keep the diff surgical.

## Merge evidence

No checkpoint commits were created. The working tree already contained unrelated
staged work (GitHub Actions composite-action extraction, Dockerfile
private-module cleanup, discovery circuit-breaker core), so committing would
have swept unrelated changes into a "fix: notifier nats retry" commit. The
RED/GREEN evidence above is the record; commit the notifier files as a scoped
change:

```
services/notifier/internal/subscribers/manager/retry.go
services/notifier/internal/tests/manager/retry_test.go
services/notifier/internal/constants/config.go
services/notifier/internal/constants/logs.go
services/notifier/main.go
```
