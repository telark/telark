package authrequest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
)

var lg = constants.GetLogger(constants.LoggerPrefixAuthRequest)

// Auth holds no Kubernetes permission: the exporter watches the AuthRequest objects, and auth asks it for the pending
// ones and reports each step back.
func Start(ctx context.Context) {
	ticker := time.NewTicker(constants.AuthRequestPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll(ctx)
		}
	}
}

// One run at a time, across replicas: another run's thaw would cut this one's freeze. The requests are listed again
// under the lock, since the run that held it may have finished them.
func poll(ctx context.Context) {
	client := clients.GetAuthRequestClient()
	// A failed list is the rest client's to log, once per outage rather than once per tick.
	pending, _ := client.Pending()
	if len(pending) == constants.DefaultInitValue {
		return
	}
	lock, err := authhelper.LockRun()
	if err != nil {
		if !errors.Is(err, authhelper.ErrRunBusy) {
			lg.Warn(fmt.Sprintf(string(constants.ErrAuthRequestRunFailed), err))
		}
		return
	}
	defer func() { _ = authhelper.UnlockRun(lock) }()
	pending, _ = client.Pending()
	for _, request := range pending {
		if ctx.Err() != nil {
			return
		}
		r := &run{request: request, lock: lock, email: strings.ToLower(strings.TrimSpace(request.Email))}
		r.process(ctx)
	}
}

// Under the run lock no run is alive, so a request found halfway is one a stopped auth left: it ends there.
func (r *run) process(ctx context.Context) {
	if !r.request.InFlight() {
		r.decide(ctx)
	}
	if r.final.Phase == constants.EmptyString {
		r.end(authrequest.PhaseFailed, constants.AuthRequestInterrupted)
	}
	r.report(r.final)
}

func (r *run) end(phase, reason string) {
	r.final = authrequest.Status{Phase: phase, Reason: reason}
}

func (r *run) failf(format string, args ...any) {
	reason := fmt.Sprintf(format, args...)
	lg.Error(fmt.Sprintf(string(constants.ErrAuthRequestStepFailed), r.request.ID, reason))
	if r.final.Phase == constants.EmptyString {
		r.end(authrequest.PhaseFailed, reason)
	}
}

// A status that didn't reach the object changes nothing in the run: the steps protect the account, not the report.
func (r *run) report(status authrequest.Status) {
	status.RequestID = r.requestID
	resp := clients.GetAuthRequestClient().SetStatus(r.request.ID, status)
	if resp.Status != http.StatusOK {
		lg.Error(fmt.Sprintf(string(constants.ErrAuthRequestReportFailed), r.request.ID, status.Phase, resp.Status, resp.Message))
	}
}

// False once auth is shutting down: the run stops where it is, and the freeze and the lock lapse on their own.
func pause(ctx context.Context, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
