package shared

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/telark/x-ware/constants"
	"github.com/telark/x-ware/shared"
)

const (
	lastIndexOffset     = 1
	wantDialsImmediate  = 1
	wantDialsAfterRetry = 3
	wantRetryLogs       = 2
	wantClosedClients   = 1
	wantMaxWaitErrors   = 1
	maxWaitWindow       = 10 * time.Millisecond
	sleepCancelWindow   = 20 * time.Millisecond
)

type conn struct {
	healthy bool
	closed  bool
}

type recordingLogger struct {
	mu     sync.Mutex
	errors []string
	infos  []string
}

func (l *recordingLogger) Error(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}

func (l *recordingLogger) Info(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}

func (l *recordingLogger) counts() (errCount, infoCount int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors), len(l.infos)
}

type dialScript struct {
	conns  []*conn
	errs   []error
	calls  int
	closed []*conn
}

func (s *dialScript) dial() (*conn, error) {
	i := s.calls
	s.calls++
	if i >= len(s.conns) {
		i = len(s.conns) - lastIndexOffset
	}
	var err error
	if i < len(s.errs) {
		err = s.errs[i]
	}
	return s.conns[i], err
}

func (s *dialScript) onFailure(c *conn, _ error) {
	if c == nil {
		return
	}
	c.closed = true
	s.closed = append(s.closed, c)
}

func policy(s *dialScript, lg shared.Logger) shared.RetryPolicy[*conn] {
	return shared.RetryPolicy[*conn]{
		Dial:       s.dial,
		Healthy:    func(c *conn) bool { return c != nil && c.healthy },
		OnFailure:  s.onFailure,
		Interval:   time.Millisecond,
		Log:        lg,
		MaxWaitMsg: "max wait %ds",
		RetryMsg:   "retry in %ds, elapsed %ds",
	}
}

func TestConnectWithRetry_ImmediateSuccess(t *testing.T) {
	good := &conn{healthy: true}
	s := &dialScript{conns: []*conn{good}}
	lg := &recordingLogger{}

	got := shared.ConnectWithRetry(t.Context(), policy(s, lg))

	if got != good {
		t.Fatalf("expected the dialed client, got %v", got)
	}
	if s.calls != wantDialsImmediate {
		t.Errorf("expected %d dial, got %d", wantDialsImmediate, s.calls)
	}
	if errCount, infoCount := lg.counts(); errCount != constants.ZeroValue || infoCount != constants.ZeroValue {
		t.Errorf("expected no logs, got %d errors and %d infos", errCount, infoCount)
	}
}

func TestConnectWithRetry_SucceedsAfterRetries(t *testing.T) {
	dead := errors.New("dial failed")
	halfOpen := &conn{}
	good := &conn{healthy: true}
	s := &dialScript{
		conns: []*conn{nil, halfOpen, good},
		errs:  []error{dead, nil, nil},
	}
	lg := &recordingLogger{}

	got := shared.ConnectWithRetry(t.Context(), policy(s, lg))

	if got != good {
		t.Fatalf("expected the healthy client, got %v", got)
	}
	if s.calls != wantDialsAfterRetry {
		t.Errorf("expected %d dials, got %d", wantDialsAfterRetry, s.calls)
	}
	if _, infoCount := lg.counts(); infoCount != wantRetryLogs {
		t.Errorf("expected %d retry logs, got %d", wantRetryLogs, infoCount)
	}
	if good.closed {
		t.Error("the returned client must not be closed")
	}
}

func TestConnectWithRetry_ClosesHalfOpenClientBeforeRetrying(t *testing.T) {
	halfOpen := &conn{}
	good := &conn{healthy: true}
	s := &dialScript{conns: []*conn{halfOpen, good}}

	if got := shared.ConnectWithRetry(t.Context(), policy(s, nil)); got != good {
		t.Fatalf("expected the healthy client, got %v", got)
	}
	if len(s.closed) != wantClosedClients || s.closed[constants.FirstIndex] != halfOpen {
		t.Fatalf("expected the half-open client to be closed once, got %d closes", len(s.closed))
	}
	if !halfOpen.closed {
		t.Error("half-open client was not closed before retrying")
	}
}

func TestConnectWithRetry_GivesUpOnMaxWait(t *testing.T) {
	s := &dialScript{conns: []*conn{{}}}
	lg := &recordingLogger{}
	p := policy(s, lg)
	p.MaxWait = maxWaitWindow

	if got := shared.ConnectWithRetry(t.Context(), p); got != nil {
		t.Fatalf("expected nil after max wait, got %v", got)
	}
	if errCount, _ := lg.counts(); errCount != wantMaxWaitErrors {
		t.Errorf("expected %d max-wait error log, got %d", wantMaxWaitErrors, errCount)
	}
	if lg.errors[constants.FirstIndex] != "max wait 0s" {
		t.Errorf("unexpected max-wait message: %q", lg.errors[constants.FirstIndex])
	}
}

func TestConnectWithRetry_StopsOnCancelledContext(t *testing.T) {
	s := &dialScript{conns: []*conn{{}}}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if got := shared.ConnectWithRetry(ctx, policy(s, nil)); got != nil {
		t.Fatalf("expected nil for a canceled context, got %v", got)
	}
	if s.calls != constants.ZeroValue {
		t.Errorf("expected no dial attempt, got %d", s.calls)
	}
}

func TestConnectWithRetry_StopsWhenContextEndsDuringSleep(t *testing.T) {
	s := &dialScript{conns: []*conn{{}}}
	p := policy(s, nil)
	p.Interval = time.Minute

	ctx, cancel := context.WithTimeout(t.Context(), sleepCancelWindow)
	defer cancel()

	start := time.Now()
	if got := shared.ConnectWithRetry(ctx, p); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	if elapsed := time.Since(start); elapsed >= time.Minute {
		t.Errorf("sleep was not interrupted by the context, waited %s", elapsed)
	}
}
