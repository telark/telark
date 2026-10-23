package bootstrap

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/kcore/crds/api"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrNotPinned        = errors.New(string(constants.ErrBootstrapNotPinned))
	ErrNoRequest        = errors.New(string(constants.ErrBootstrapNoRequest))
	ErrNotPending       = errors.New(string(constants.ErrBootstrapNotPending))
	ErrExpired          = errors.New(string(constants.ErrBootstrapExpired))
	ErrNotEligible      = errors.New(string(constants.ErrBootstrapNotEligible))
	ErrEnrolled         = errors.New(string(constants.ErrBootstrapEnrolled))
	ErrReissue          = errors.New(string(constants.ErrBootstrapReissueOver))
	ErrNoTarget         = errors.New(string(constants.ErrBootstrapNoTarget))
	ErrReason           = errors.New(string(constants.ErrBootstrapReason))
	ErrEmail            = errors.New(string(constants.ErrBootstrapEmailRequired))
	ErrRecoveryApproval = errors.New(string(constants.ErrBootstrapRecoveryApproval))
	ErrKeep             = errors.New(string(constants.ErrBootstrapKeep))

	// A request's cause, by its reason, else its state; break-glass for the rest.
	causes = map[string]string{
		telarkconfig.BootstrapStateApproved:  constants.BootstrapCausePasskey,
		telarkconfig.BootstrapReasonDenied:   constants.BootstrapCausePasskey,
		telarkconfig.BootstrapReasonSignedIn: constants.BootstrapCauseSignIn,
		telarkconfig.BootstrapReasonExpired:  constants.BootstrapCauseSeventyTwoH,
	}
)

// One open request at a time: a run while one is open, or within 24 hours after a replacement completed while its
// account has no passkey, gets that one back, so a lost token costs no new 72 hours.
func Open(optimizer *performance.Optimizer, email string) (*Outcome, error) {
	email = userutils.NormalizeEmail(email)
	if email == constants.EmptyString {
		return &Outcome{}, ErrEmail
	}
	unlock := lockRequests()
	defer unlock()
	accountID, err := accountOf(email)
	if err != nil {
		return &Outcome{}, err
	}
	outcome := &Outcome{}
	status, err := update(func(status *telarkconfig.BootstrapStatus) (bool, error) {
		*outcome = Outcome{}
		if !status.Pinned() {
			return false, ErrNotPinned
		}
		at := time.Now().UTC()
		// The pinned account is found by its current email, which may differ from the one it was pinned with.
		recovery := accountID != constants.EmptyString && accountID == status.UserID
		if newest := status.Newest(); newest != nil {
			if reuse, err := reusable(optimizer, status, newest, concerns(newest, recovery, email), at, outcome); err != nil || reuse {
				return false, err
			}
		}
		request, err := newRequest(recovery, email, at)
		if err != nil {
			return false, err
		}
		status.Requests = slices.Insert(status.Requests, constants.DefaultInitValue, request)
		status.Requests = status.Requests[:min(len(status.Requests), constants.BootstrapRequestsKept)]
		outcome.Result = eps.RequestResult{Request: request}
		outcome.Changed = append(outcome.Changed, request)
		return true, nil
	})
	outcome = pinnedTo(outcome, status, err)
	if err != nil {
		return outcome, err
	}
	logChanges(outcome)
	return outcome, dropEnded(optimizer, status.UserID, outcome.Changed)
}

// After its 72-hour mark, an open request the bootstrap account signed in during ends here, and a new one opens.
// Only a completed replacement re-issues, for the account it concerns: a completed recovery's account has a passkey.
func reusable(
	optimizer *performance.Optimizer, status *telarkconfig.BootstrapStatus, newest *telarkconfig.BootstrapRequest, mine bool,
	at time.Time, outcome *Outcome,
) (bool, error) {
	switch {
	case newest.Open():
		return reuseOpen(optimizer, status, newest, at, outcome)
	case newest.State == telarkconfig.BootstrapStateCompleted && newest.Kind == telarkconfig.BootstrapKindReplace && mine:
		if again, err := reissuable(newest, at); err != nil || !again {
			return false, err
		}
		enrolled, err := hasPasskey(status.UserID)
		if err != nil || enrolled {
			return false, err
		}
		outcome.Result, outcome.Issued = eps.RequestResult{Request: *newest, Eligible: true}, true
		return true, nil
	}
	return false, nil
}

func reuseOpen(
	optimizer *performance.Optimizer, status *telarkconfig.BootstrapStatus, newest *telarkconfig.BootstrapRequest, at time.Time,
	outcome *Outcome,
) (bool, error) {
	eligible, expired, err := judge(status, newest, at)
	if err != nil {
		return false, err
	}
	if expired {
		return false, endRequest(optimizer, status.UserID, newest, telarkconfig.BootstrapReasonExpired, at, outcome)
	}
	outcome.Result = eps.RequestResult{Request: *newest, Eligible: eligible}
	return true, nil
}

func reissuable(request *telarkconfig.BootstrapRequest, at time.Time) (bool, error) {
	completed, err := time.Parse(time.RFC3339, request.CompletedAt)
	if err != nil {
		return false, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
	}
	return at.Before(completed.Add(telarkconfig.BootstrapReissueWindow)), nil
}

// A recovery when the email is the pinned account's, else a replacement that moves the pin to that email.
func newRequest(recovery bool, email string, at time.Time) (telarkconfig.BootstrapRequest, error) {
	request := telarkconfig.BootstrapRequest{State: telarkconfig.BootstrapStatePending, OpenedAt: stamp(at)}
	random := make([]byte, constants.BootstrapRequestIDBytes)
	if _, err := rand.Read(random); err != nil {
		return request, err
	}
	request.ID = hex.EncodeToString(random)
	request.Kind = telarkconfig.BootstrapKindRecover
	if !recovery {
		request.Kind, request.TargetEmail = telarkconfig.BootstrapKindReplace, email
	}
	return request, nil
}

// A recovery concerns the pinned account; a replacement, the email it names.
func concerns(request *telarkconfig.BootstrapRequest, recovery bool, email string) bool {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return request.TargetEmail == email
	}
	return recovery
}

// The account an email names, or none.
func accountOf(email string) (string, error) {
	item, err := userutils.FindUserByEmail(email)
	if errors.Is(err, userutils.ErrUserNotFound) {
		return constants.EmptyString, nil
	}
	if err != nil {
		return constants.EmptyString, err
	}
	return item.GetName(), nil
}

// An owner who could approve a recovery still has a passkey, so an approval would only ever help someone else.
func Approve(id string) (*Outcome, error) {
	outcome, _, err := transition(id, func(
		_ *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, _ *Outcome,
	) (bool, error) {
		switch {
		case request.Kind == telarkconfig.BootstrapKindRecover:
			return false, ErrRecoveryApproval
		case request.State != telarkconfig.BootstrapStatePending:
			return false, ErrNotPending
		}
		opened, err := time.Parse(time.RFC3339, request.OpenedAt)
		if err != nil {
			return false, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
		}
		if !at.Before(opened.Add(telarkconfig.BootstrapRequestWait)) {
			return false, ErrExpired
		}
		request.State, request.ApprovedAt = telarkconfig.BootstrapStateApproved, stamp(at)
		return true, nil
	})
	outcome.Result.Eligible = err == nil
	if err == nil {
		logRequest(outcome.Result.Request, outcome.Result.Request.State)
	}
	return outcome, err
}

// A sign-in cancels a pending request only, so the bootstrap account's denial is the one way to take back an approval.
func Cancel(optimizer *performance.Optimizer, id, reason string) (*Outcome, error) {
	if reason != telarkconfig.BootstrapReasonSignedIn && reason != telarkconfig.BootstrapReasonDenied {
		return &Outcome{}, ErrReason
	}
	unlock := lockRequests()
	defer unlock()
	outcome, status, err := transition(id, func(
		status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
	) (bool, error) {
		denied := request.State == telarkconfig.BootstrapStateApproved && reason == telarkconfig.BootstrapReasonDenied
		if request.State != telarkconfig.BootstrapStatePending && !denied {
			return false, ErrNotPending
		}
		err := endRequest(optimizer, status.UserID, request, reason, at, outcome)
		return err == nil, err
	})
	if err != nil {
		return outcome, err
	}
	logChanges(outcome)
	return outcome, dropEnded(optimizer, status.UserID, outcome.Changed)
}

// A recovery goes link first, wipe after: only the pass that keeps the passkey its link registered completes it. A
// replacement completes and moves the pin, and completes again, setting the markers once more, while it may re-issue.
func Complete(optimizer *performance.Optimizer, id string, revoke bool, keep string) (*Outcome, error) {
	unlock := lockRequests()
	defer unlock()
	outcome, status, err := transition(id, func(
		status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
	) (bool, error) {
		if request.Kind == telarkconfig.BootstrapKindRecover {
			return false, eligibleNow(status, request, at)
		}
		return completeReplacement(status, request, at, outcome)
	})
	if err != nil {
		return outcome, dropUnkept(optimizer, status, keep, err)
	}
	logChanges(outcome)
	request := status.Newest()
	switch {
	case request.Kind == telarkconfig.BootstrapKindReplace:
		return settle(optimizer, status, request, outcome)
	case keep == constants.EmptyString:
		return prepare(optimizer, status.UserID, request, revoke, outcome)
	}
	return keepOnly(optimizer, status.UserID, request, keep, outcome)
}

func completeReplacement(
	status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
) (bool, error) {
	if request.State == telarkconfig.BootstrapStateCompleted {
		return false, completeAgain(status, request, at)
	}
	if err := eligibleNow(status, request, at); err != nil {
		return false, err
	}
	targetID, err := replacementID(request.TargetEmail)
	if err != nil {
		return false, err
	}
	status.UserID, status.Email, status.PinnedAt = targetID, request.TargetEmail, stamp(at)
	request.State, request.CompletedAt = telarkconfig.BootstrapStateCompleted, stamp(at)
	outcome.Changed = append(outcome.Changed, *request)
	return true, nil
}

// A completed replacement completes again within 24 hours, while its account has no passkey.
func completeAgain(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time) error {
	again, err := reissuable(request, at)
	if err == nil && !again {
		err = ErrReissue
	}
	if err != nil {
		return err
	}
	enrolled, err := hasPasskey(status.UserID)
	if err == nil && enrolled {
		err = ErrEnrolled
	}
	return err
}

// A replacement moves the marker with the pin, and the previous account keeps its roles, so the new bootstrap account
// can demote it.
func settle(
	optimizer *performance.Optimizer, status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, outcome *Outcome,
) (*Outcome, error) {
	err := setMarker(optimizer, status.UserID, true)
	if err == nil {
		err = settleMarkers(optimizer, status.UserID)
	}
	if err != nil {
		return outcome, err
	}
	enrolled, err := hasPasskey(status.UserID)
	outcome.Result = eps.RequestResult{Request: *request, Eligible: true, Enrolled: enrolled}
	return outcome, err
}

// The passes before the link: the account's sessions end, and the last one, made while the account is frozen
// (revoke), also deletes the passkeys saved after the mark, which a link of a run that stopped saved.
func prepare(
	optimizer *performance.Optimizer, userID string, request *telarkconfig.BootstrapRequest, revoke bool, outcome *Outcome,
) (*Outcome, error) {
	outcome.Result = eps.RequestResult{Request: *request, Eligible: true}
	err := sessionutils.PurgeSessionsForUser(userID)
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserSession)
	if err == nil && revoke {
		if err = revokeAfterMark(optimizer, userID, request); err == nil {
			outcome.Issued = true
			logChanges(outcome)
		}
	}
	return outcome, err
}

// Never the account's last passkeys: the ones saved after the mark go only while one from before it (or unparsable) stays.
func revokeAfterMark(optimizer *performance.Optimizer, userID string, request *telarkconfig.BootstrapRequest) error {
	mark, err := markOf(request)
	if err != nil {
		return err
	}
	passkeys, err := passkeyutils.FindPasskeysByUserID(userID)
	if err != nil || !slices.ContainsFunc(passkeys, func(passkey unstructured.Unstructured) bool { return !savedAfter(passkey, mark) }) {
		return err
	}
	return dropAfterMark(optimizer, userID, request, constants.EmptyString)
}

// The last pass keeps the link's passkey and removes every session, passkey sign-in and then other passkey: one that
// fails leaves the request open for the next run, and the account its passkeys if it failed before those.
func keepOnly(
	optimizer *performance.Optimizer, userID string, request *telarkconfig.BootstrapRequest, keep string, outcome *Outcome,
) (*Outcome, error) {
	if err := keepable(userID, request, keep); err != nil {
		return outcome, err
	}
	err := sessionutils.PurgeSessionsForUser(userID)
	if err == nil {
		err = dropPasskeyIdentities(optimizer, userID, keep)
	}
	if err == nil {
		err = passkeyutils.PurgePasskeysWhere(userID, func(passkey unstructured.Unstructured) bool { return credentialOf(passkey) != keep })
	}
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserPasskey)
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserSession)
	if err != nil {
		return outcome, err
	}
	completed, _, err := transition(request.ID, completeRecovery)
	completed.Result.Eligible = err == nil
	if err == nil {
		logChanges(completed)
	}
	return completed, err
}

// A sign-in stamped during the deletes doesn't stop it: the request's end would then take the passkey it kept, the last.
func completeRecovery(
	_ *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
) (bool, error) {
	if !request.Open() {
		return false, ErrNotPending
	}
	request.State, request.CompletedAt = telarkconfig.BootstrapStateCompleted, stamp(at)
	outcome.Changed = append(outcome.Changed, *request)
	return true, nil
}

// Only a passkey of the pinned account saved after the mark can be the one the recovery's link registered.
func keepable(userID string, request *telarkconfig.BootstrapRequest, keep string) error {
	mark, err := markOf(request)
	if err != nil {
		return err
	}
	passkeys, err := passkeyutils.FindPasskeysByUserID(userID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(passkeys, func(passkey unstructured.Unstructured) bool {
		return credentialOf(passkey) == keep && savedAfter(passkey, mark)
	}) {
		return ErrKeep
	}
	return nil
}

// A recovery that ends without completing takes along what a link saved after its mark (a run that stopped while waiting
// left it, and the bootstrap sign-in refuses it only while the recovery is open), before the end is written.
func endRequest(
	optimizer *performance.Optimizer, userID string, request *telarkconfig.BootstrapRequest, reason string, at time.Time, outcome *Outcome,
) error {
	if request.Kind == telarkconfig.BootstrapKindRecover {
		if err := revokeAfterMark(optimizer, userID, request); err != nil {
			return err
		}
	}
	end(request, reason, at)
	outcome.Changed = append(outcome.Changed, *request)
	return nil
}

// Again once the end is written, for a passkey a link saved meanwhile.
func dropEnded(optimizer *performance.Optimizer, userID string, requests []telarkconfig.BootstrapRequest) error {
	errs := make([]error, constants.DefaultInitValue, len(requests))
	for i := range requests {
		if requests[i].Kind == telarkconfig.BootstrapKindRecover && requests[i].State == telarkconfig.BootstrapStateCanceled {
			errs = append(errs, revokeAfterMark(optimizer, userID, &requests[i]))
		}
	}
	return errors.Join(errs...)
}

// A link's registration that lands after its recovery ended would sign in with nothing left to refuse it: the refused
// keep pass deletes that passkey, and no other.
func dropUnkept(optimizer *performance.Optimizer, status *telarkconfig.BootstrapStatus, keep string, refusal error) error {
	if keep == constants.EmptyString || !errors.Is(refusal, ErrNotPending) {
		return refusal
	}
	request := status.Newest()
	if request.Kind != telarkconfig.BootstrapKindRecover || request.State != telarkconfig.BootstrapStateCanceled {
		return refusal
	}
	if err := dropAfterMark(optimizer, status.UserID, request, keep); err != nil {
		return err
	}
	return refusal
}

// The passkeys saved after the mark, or only the one with credential when it is set.
func dropAfterMark(optimizer *performance.Optimizer, userID string, request *telarkconfig.BootstrapRequest, credential string) error {
	mark, err := markOf(request)
	if err != nil {
		return err
	}
	err = passkeyutils.PurgePasskeysWhere(userID, func(passkey unstructured.Unstructured) bool {
		return savedAfter(passkey, mark) && (credential == constants.EmptyString || credentialOf(passkey) == credential)
	})
	cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserPasskey)
	return err
}

// By the stamp auth wrote in the spec, which a restore keeps while it resets the metadata's. One that doesn't parse is
// never after the mark, so it is never deleted or kept as the link's.
func savedAfter(passkey unstructured.Unstructured, mark time.Time) bool {
	value, _, _ := unstructured.NestedString(passkey.Object, constants.SpecField, constants.FieldCreationTimestamp)
	created, err := time.Parse(time.RFC3339, value)
	return err == nil && !created.Before(mark)
}

func markOf(request *telarkconfig.BootstrapRequest) (time.Time, error) {
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
	}
	return opened.Add(telarkconfig.BootstrapRequestWait), nil
}

func credentialOf(passkey unstructured.Unstructured) string {
	id, _, _ := unstructured.NestedString(passkey.Object, constants.SpecField, constants.FieldCredentialID)
	return id
}

// Open, Cancel and Complete run one at a time (the exporter runs one replica), so no end deletes what a keep pass keeps.
func lockRequests() (unlock func()) {
	lock := concurrency.GetLock(constants.BootstrapRequestsLock)
	lock.Lock()
	return lock.Unlock
}

// change runs on the newest request, which must be id, and says whether to write the status it changed.
func transition(
	id string,
	change func(*telarkconfig.BootstrapStatus, *telarkconfig.BootstrapRequest, time.Time, *Outcome) (bool, error),
) (*Outcome, *telarkconfig.BootstrapStatus, error) {
	outcome := &Outcome{}
	status, err := update(func(status *telarkconfig.BootstrapStatus) (bool, error) {
		*outcome = Outcome{}
		request := status.Newest()
		if request == nil || request.ID != id {
			return false, ErrNoRequest
		}
		write, err := change(status, request, time.Now().UTC(), outcome)
		outcome.Result.Request = *request
		return write, err
	})
	return pinnedTo(outcome, status, err), status, err
}

func eligibleNow(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time) error {
	if !request.Open() {
		return ErrNotPending
	}
	eligible, _, err := judge(status, request, at)
	if err == nil && !eligible {
		err = ErrNotEligible
	}
	return err
}

// Eligible: after the 72-hour mark with no sign-in by the bootstrap account since the request opened, or a replacement
// approved before it. A sign-in after the mark expires it instead.
func judge(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time) (eligible, expired bool, err error) {
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		return false, false, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
	}
	if at.Before(opened.Add(telarkconfig.BootstrapRequestWait)) {
		// An approval a recovery got before recoveries lost theirs counts for nothing.
		return request.Kind == telarkconfig.BootstrapKindReplace && request.State == telarkconfig.BootstrapStateApproved, false, nil
	}
	signedIn, err := signedInSince(status.UserID, opened)
	return err == nil && !signedIn, signedIn, err
}

// Auth stamps lastLoginAt at each sign-in, so a cancel call that never arrived still counts. A deleted account never
// signs in, and a stamp that doesn't parse counts as a sign-in.
func signedInSince(userID string, since time.Time) (bool, error) {
	user, err := loadUser(userID)
	if err != nil || user == nil || user.Status.LastLoginAt == nil {
		return false, err
	}
	at, err := time.Parse(time.RFC3339, *user.Status.LastLoginAt)
	return err != nil || !at.Before(since), nil
}

func hasPasskey(userID string) (bool, error) {
	passkeys, err := passkeyutils.FindPasskeysByUserID(userID)
	return len(passkeys) > constants.DefaultInitValue, err
}

// break-glass creates and promotes the account before it completes the request.
func replacementID(email string) (string, error) {
	id, err := accountOf(email)
	if err == nil && id == constants.EmptyString {
		return id, ErrNoTarget
	}
	return id, err
}

// Read and written under the user's lock, which the user patch takes too, so no identity added meanwhile is lost.
func dropPasskeyIdentities(optimizer *performance.Optimizer, userID, keep string) error {
	lock := concurrency.GetLock(userID)
	lock.Lock()
	defer lock.Unlock()
	user, err := loadUser(userID)
	if err != nil || user == nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(user.Identities), func(identity *userdata.UserIdentity) bool {
		return identity != nil && identity.Provider == constants.IdentityProviderPasskey && identity.Subject != keep
	})
	if len(kept) == len(user.Identities) {
		return nil
	}
	result := api.PatchCustomResource(metadata.UserMetadata, userID,
		map[string]any{constants.SpecField: map[string]any{constants.FieldIdentities: kept}})
	userutils.InvalidateUserCaches(optimizer, userID)
	if result.Status != http.StatusOK {
		return fmt.Errorf(string(constants.ErrBootstrapIdentitiesWrite), userID, result.Error)
	}
	return nil
}

func end(request *telarkconfig.BootstrapRequest, reason string, at time.Time) {
	request.State, request.Reason, request.CanceledAt = telarkconfig.BootstrapStateCanceled, reason, stamp(at)
}

// One line per change: the bell's notices can be cleared, and the record shows only each request's last state.
func logChanges(outcome *Outcome) {
	for _, request := range outcome.Changed {
		event := request.State
		if request.Reason != constants.EmptyString {
			event = fmt.Sprintf(constants.BootstrapLogCanceled, request.State, request.Reason)
		}
		logRequest(request, event)
	}
	if outcome.Issued {
		event := constants.BootstrapLogLinkIssued
		if outcome.Result.Request.Kind == telarkconfig.BootstrapKindReplace {
			event = constants.BootstrapLogLinkReissued
		}
		logRequest(outcome.Result.Request, event)
	}
}

func logRequest(request telarkconfig.BootstrapRequest, event string) {
	cause, found := causes[cmp.Or(request.Reason, request.State)]
	if !found {
		cause = constants.BootstrapCauseBreakGlass
	}
	lg.Info(fmt.Sprintf(string(constants.InfBootstrapRequest), request.ID, request.Kind, event, cause))
}

// A failed update wrote nothing, so it changed no request a notice could report.
func pinnedTo(outcome *Outcome, status *telarkconfig.BootstrapStatus, err error) *Outcome {
	if err != nil {
		outcome.Changed, outcome.Issued = nil, false
	}
	if status != nil {
		outcome.Pinned = status.UserID
	}
	return outcome
}

func stamp(at time.Time) string {
	return at.Format(time.RFC3339)
}
