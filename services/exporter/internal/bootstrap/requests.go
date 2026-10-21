package bootstrap

import (
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
	ErrNotPinned   = errors.New(string(constants.ErrBootstrapNotPinned))
	ErrNoRequest   = errors.New(string(constants.ErrBootstrapNoRequest))
	ErrNotPending  = errors.New(string(constants.ErrBootstrapNotPending))
	ErrExpired     = errors.New(string(constants.ErrBootstrapExpired))
	ErrNotEligible = errors.New(string(constants.ErrBootstrapNotEligible))
	ErrEnrolled    = errors.New(string(constants.ErrBootstrapEnrolled))
	ErrReissue     = errors.New(string(constants.ErrBootstrapReissueOver))
	ErrNoTarget    = errors.New(string(constants.ErrBootstrapNoTarget))
	ErrReason      = errors.New(string(constants.ErrBootstrapReason))
	ErrEmail       = errors.New(string(constants.ErrBootstrapEmailRequired))
)

// One open request at a time: a run while one is open, or within 24 hours after one completed with no passkey
// enrolled since, gets that one back, so a lost token costs no new 72 hours.
func Open(email string) (*Outcome, error) {
	email = userutils.NormalizeEmail(email)
	if email == constants.EmptyString {
		return &Outcome{}, ErrEmail
	}
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
			if reuse, err := reusable(status, newest, concerns(newest, recovery, email), at, outcome); err != nil || reuse {
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
	return pinnedTo(outcome, status, err), err
}

// After its 72-hour mark, an open request the bootstrap account signed in during ends here, and a new one opens.
// A completed one re-issues only for the account it concerns.
func reusable(
	status *telarkconfig.BootstrapStatus, newest *telarkconfig.BootstrapRequest, mine bool, at time.Time, outcome *Outcome,
) (bool, error) {
	switch {
	case newest.Open():
		eligible, expired, err := judge(status, newest, at)
		if err != nil || expired {
			if expired {
				end(newest, telarkconfig.BootstrapReasonExpired, at)
				outcome.Changed = append(outcome.Changed, *newest)
			}
			return false, err
		}
		outcome.Result = eps.RequestResult{Request: *newest, Eligible: eligible}
		return true, nil
	case newest.State == telarkconfig.BootstrapStateCompleted && mine:
		if again, err := reissuable(newest, at); err != nil || !again {
			return false, err
		}
		enrolled, err := enrolledSince(status, newest)
		if err != nil || enrolled {
			return false, err
		}
		outcome.Result, outcome.Reissued = eps.RequestResult{Request: *newest, Eligible: true}, true
		return true, nil
	}
	return false, nil
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

func Approve(id string) (*Outcome, error) {
	outcome, _, err := transition(id, func(
		_ *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, _ *Outcome,
	) (bool, error) {
		if request.State != telarkconfig.BootstrapStatePending {
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
	return outcome, err
}

// A sign-in cancels a pending request only, so the bootstrap account's denial is the one way to take back an approval.
func Cancel(id, reason string) (*Outcome, error) {
	if reason != telarkconfig.BootstrapReasonSignedIn && reason != telarkconfig.BootstrapReasonDenied {
		return &Outcome{}, ErrReason
	}
	outcome, _, err := transition(id, func(
		_ *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
	) (bool, error) {
		denied := request.State == telarkconfig.BootstrapStateApproved && reason == telarkconfig.BootstrapReasonDenied
		if request.State != telarkconfig.BootstrapStatePending && !denied {
			return false, ErrNotPending
		}
		end(request, reason, at)
		outcome.Changed = append(outcome.Changed, *request)
		return true, nil
	})
	return outcome, err
}

// The status is written first, so a run that fails halfway is finished by the next one: a completed request with no
// passkey enrolled since (or with revoke, any) completes again, purging (recovery) or setting the markers
// (replacement) once more.
func Complete(optimizer *performance.Optimizer, id string, revoke bool) (*Outcome, error) {
	outcome, status, err := transition(id, func(
		status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, outcome *Outcome,
	) (bool, error) {
		switch {
		case request.State == telarkconfig.BootstrapStateCompleted:
			return false, completeAgain(status, request, at, revoke)
		case !request.Open():
			return false, ErrNotPending
		}
		eligible, _, err := judge(status, request, at)
		if err != nil {
			return false, err
		}
		if !eligible {
			return false, ErrNotEligible
		}
		if request.Kind == telarkconfig.BootstrapKindReplace {
			targetID, err := replacementID(request.TargetEmail)
			if err != nil {
				return false, err
			}
			status.UserID, status.Email, status.PinnedAt = targetID, request.TargetEmail, stamp(at)
		}
		request.State, request.CompletedAt = telarkconfig.BootstrapStateCompleted, stamp(at)
		outcome.Changed = append(outcome.Changed, *request)
		return true, nil
	})
	if err != nil {
		return outcome, err
	}
	request := status.Newest()
	if err := settle(optimizer, status, request); err != nil {
		return outcome, err
	}
	enrolled, err := enrolledSince(status, request)
	outcome.Result = eps.RequestResult{Request: *request, Eligible: true, Enrolled: enrolled}
	return outcome, err
}

// A completed request completes again, purging once more, within 24 hours and while no passkey was enrolled since.
// With revoke, break-glass's last pass while the account is frozen, a passkey enrolled since is purged too: one that a
// request in flight when the recovery began saved after the first pass.
func completeAgain(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time, revoke bool) error {
	again, err := reissuable(request, at)
	if err == nil && !again {
		err = ErrReissue
	}
	if err != nil || revoke {
		return err
	}
	enrolled, err := enrolledSince(status, request)
	if err == nil && enrolled {
		err = ErrEnrolled
	}
	return err
}

// A recovery ends every way into the account, and the cached passkey and session lists forget them at once; a
// replacement moves the marker with the pin, and the previous account keeps its roles, so the new bootstrap account
// can demote it.
func settle(optimizer *performance.Optimizer, status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) error {
	if request.Kind == telarkconfig.BootstrapKindRecover {
		err := errors.Join(
			passkeyutils.PurgePasskeysForUser(status.UserID),
			sessionutils.PurgeSessionsForUser(status.UserID),
			dropPasskeyIdentities(optimizer, status.UserID),
		)
		cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserPasskey)
		cache.InvalidateAllResourceCaches(optimizer, constants.ResourceUserSession)
		return err
	}
	if err := setMarker(optimizer, status.UserID, true); err != nil {
		return err
	}
	return settleMarkers(optimizer, status.UserID)
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

// Eligible: approved before the 72-hour mark, or pending or approved after it with no sign-in by the bootstrap
// account since the request opened. A sign-in after the mark expires it instead.
func judge(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, at time.Time) (eligible, expired bool, err error) {
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		return false, false, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
	}
	if at.Before(opened.Add(telarkconfig.BootstrapRequestWait)) {
		return request.State == telarkconfig.BootstrapStateApproved, false, nil
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

// A recovery counts the passkeys created once it completed, not one a failed purge left; a replacement counts any.
func enrolledSince(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) (bool, error) {
	var since time.Time
	if request.Kind == telarkconfig.BootstrapKindRecover {
		completed, err := time.Parse(time.RFC3339, request.CompletedAt)
		if err != nil {
			return false, fmt.Errorf(string(constants.ErrBootstrapStatusInvalid), err)
		}
		since = completed
	}
	passkeys, err := passkeyutils.FindPasskeysByUserID(status.UserID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(passkeys, func(passkey unstructured.Unstructured) bool {
		return !passkey.GetCreationTimestamp().Time.Before(since)
	}), nil
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
func dropPasskeyIdentities(optimizer *performance.Optimizer, userID string) error {
	lock := concurrency.GetLock(userID)
	lock.Lock()
	defer lock.Unlock()
	user, err := loadUser(userID)
	if err != nil || user == nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(user.Identities), func(identity *userdata.UserIdentity) bool {
		return identity != nil && identity.Provider == constants.IdentityProviderPasskey
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

// A failed update wrote nothing, so it changed no request a notice could report.
func pinnedTo(outcome *Outcome, status *telarkconfig.BootstrapStatus, err error) *Outcome {
	if err != nil {
		outcome.Changed, outcome.Reissued = nil, false
	}
	if status != nil {
		outcome.Pinned = status.UserID
	}
	return outcome
}

func stamp(at time.Time) string {
	return at.Format(time.RFC3339)
}
