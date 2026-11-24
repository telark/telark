package authrequest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/authrequest"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

// Link first, wipe after: the account keeps its passkeys until one registers with the link this run seals, and while
// the run waits for that, up to the link's window, the freeze lets only that registration through.
func (r *run) recoverAccount(ctx context.Context, userID string) {
	r.report(authrequest.Status{
		Phase:  authrequest.PhaseRecovering,
		Reason: fmt.Sprintf(constants.AuthRequestRecovering, r.email),
	})
	if !r.revokeWaysIn(ctx, userID) {
		return
	}
	issuedAt := time.Now()
	token, expiresAt, ok := r.issueLink(userID)
	if !ok || !r.extendRun() ||
		!r.redis(func() error { return authhelper.FreezeForLink(userID, token, constants.RecoveryLinkWindow) }) ||
		!r.linkReady(token, expiresAt) {
		return
	}
	keep, ended, ok := r.await(ctx, userID, token, issuedAt)
	switch {
	case !ok:
	case keep == nil:
		r.expire(ctx, userID, issuedAt, ended)
	default:
		r.finishRecovery(userID, keep)
	}
}

// A write sent before the freeze lands within RecoveryHold, so the last drop and revoke leave no way in but the link.
// A failure before anything was revoked thaws at once; a later one leaves the freeze to lapse.
func (r *run) revokeWaysIn(ctx context.Context, userID string) bool {
	if !r.freeze(userID) {
		return false
	}
	frozenAt := time.Now()
	if !r.dropLinks(userID) || !r.refreeze(userID) {
		r.thaw(userID)
		return false
	}
	if _, status := r.complete(bootstrapeps.CompleteBody{}); status != http.StatusOK {
		if refused(status) {
			r.thaw(userID)
		}
		return false
	}
	if !r.hold(ctx, userID, frozenAt) || !r.dropLinks(userID) || !r.refreeze(userID) {
		return false
	}
	_, status := r.complete(bootstrapeps.CompleteBody{Revoke: true})
	return status == http.StatusOK
}

// Checked before the wait too, so a lapse stops the run before it waits.
func (r *run) hold(ctx context.Context, userID string, frozenAt time.Time) bool {
	if !r.refreeze(userID) || !r.extendRun() || !pause(ctx, time.Until(frozenAt.Add(constants.RecoveryHold))) {
		return false
	}
	return r.refreeze(userID) && r.extendRun()
}

// Until a passkey registered since the link shows, the link's freeze lapses with its window, or the request ends (a
// sign-in begun before the freeze cancels it late). A lost run lock or a Redis that can't answer stops the run.
func (r *run) await(ctx context.Context, userID, token string, issuedAt time.Time) (keep *authdata.Passkey, ended, ok bool) {
	for {
		held := false
		if !pause(ctx, constants.RecoveryPollInterval) || !r.extendRun() || !r.redis(func() (err error) {
			held, err = authhelper.FrozenForLink(userID, token)
			return err
		}) {
			return nil, false, false
		}
		// A lookup that failed is tried again at the next round.
		keep, _ = registered(userID, issuedAt)
		if ended = !r.stillOpen(); keep != nil || !held || ended {
			return keep, ended, true
		}
	}
}

// While the run waits, the freeze lets only the link's registration through, so a passkey created since the link was
// issued is the one it registered.
func registered(userID string, issuedAt time.Time) (*authdata.Passkey, error) {
	passkeys, err := clients.GetPasskeyClient().GetAllPasskeysByUser(userID)
	if err != nil {
		return nil, err
	}
	since := issuedAt.Truncate(time.Second)
	for _, passkey := range passkeys {
		if passkey == nil || passkey.CreationTimestamp == nil {
			continue
		}
		if created, err := time.Parse(time.RFC3339, *passkey.CreationTimestamp); err == nil && !created.Before(since) {
			return passkey, nil
		}
	}
	return nil, nil
}

// A failed read counts as open: the next round reads it again.
func (r *run) stillOpen() bool {
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		return true
	}
	newest := status.Newest()
	return newest != nil && newest.ID == r.requestID && newest.Open()
}

// The passkey the link registered stays; the exporter removes every other passkey, session and passkey sign-in, then
// completes the request. A refusal thaws too: the request may have ended meanwhile, and the exporter then deleted it.
func (r *run) finishRecovery(userID string, keep *authdata.Passkey) {
	if !r.freeze(userID) || !r.dropLinks(userID) || !r.refreeze(userID) {
		return
	}
	if _, status := r.complete(bootstrapeps.CompleteBody{Keep: keep.CredentialID}); status != http.StatusOK {
		if refused(status) {
			r.thaw(userID)
		}
		return
	}
	if r.thaw(userID) {
		r.end(authrequest.PhaseCompleted, fmt.Sprintf(constants.AuthRequestRecovered, r.requestID, r.email))
	}
}

// The window or the request ended with no registration seen: frozen again with the link dropped, a last look once a
// write sent before then has landed still hands a late registration to the exporter; else the account keeps its passkeys.
func (r *run) expire(ctx context.Context, userID string, issuedAt time.Time, ended bool) {
	if !r.freeze(userID) {
		return
	}
	frozenAt := time.Now()
	if !r.dropLinks(userID) || !r.hold(ctx, userID, frozenAt) {
		return
	}
	keep, err := r.lastLook(ctx, userID, issuedAt)
	if keep != nil {
		r.finishRecovery(userID, keep)
		return
	}
	if !r.thaw(userID) {
		return
	}
	switch {
	case err != nil:
		r.failf(string(constants.ErrAuthRequestRunFailed), err)
	case ended:
		r.end(authrequest.PhaseCanceled, fmt.Sprintf(constants.AuthRequestEnded, r.requestID, r.email))
	default:
		r.end(authrequest.PhaseExpired, fmt.Sprintf(constants.AuthRequestLinkExpired, r.requestID))
	}
}

// The exporter purged the request's passkeys before this registration landed, so a failed read is retried under a
// renewed freeze rather than leaving the passkey on a closed request.
func (r *run) lastLook(ctx context.Context, userID string, issuedAt time.Time) (*authdata.Passkey, error) {
	keep, err := registered(userID, issuedAt)
	for attempt := constants.DefaultIncrementValue; err != nil && attempt < constants.RecoveryLastLookAttempts; attempt++ {
		if !pause(ctx, constants.RecoveryPollInterval) || !r.freeze(userID) {
			return nil, err
		}
		keep, err = registered(userID, issuedAt)
	}
	return keep, err
}

// A refusal (4xx) is the exporter's finished answer, with no write of it still in flight for the freeze to wait out.
func refused(status int) bool {
	return status >= http.StatusBadRequest && status < http.StatusInternalServerError
}

func (r *run) extendRun() bool {
	return r.redis(func() error { return authhelper.ExtendRun(r.lock) })
}

// A freeze or a thaw only while the run still holds its lock: once that lapsed, the freeze may be the next run's.
func (r *run) freeze(userID string) bool {
	return r.extendRun() && r.redis(func() error { return authhelper.Freeze(userID) })
}

func (r *run) refreeze(userID string) bool {
	return r.redis(func() error { return authhelper.Refreeze(userID) })
}

func (r *run) thaw(userID string) bool {
	return r.extendRun() && r.redis(func() error { return authhelper.Unfreeze(userID) })
}

// The account's links and its open ceremonies.
func (r *run) dropLinks(userID string) bool {
	return r.redis(func() error {
		return errors.Join(authhelper.DropLinks(userID), webauthnhelper.DropCeremonies(userID))
	})
}

func (r *run) redis(op func() error) bool {
	err := op()
	if err != nil {
		r.failf(string(constants.ErrAuthRequestRunFailed), err)
	}
	return err == nil
}
