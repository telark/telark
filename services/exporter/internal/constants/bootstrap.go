package constants

import (
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
)

const (
	BootstrapRequestIDBytes = 8
	BootstrapRequestsKept   = 20
	BootstrapWriteAttempts  = 5
	IdentityProviderPasskey = "passkey"
	PrefixBootstrap         = "Bootstrap: "
	BootstrapDisplayName    = "Bootstrap account"
	// No object name holds a colon, so no resource's lock shares this one.
	BootstrapRequestsLock = "bootstrap:requests"
)

const (
	ErrBootstrapPinned          errors.Error = "the bootstrap account is pinned: only a completed break-glass replacement moves the marker"
	ErrBootstrapNotPinned       errors.Error = "no bootstrap account is pinned yet, so break-glass acts at once"
	ErrBootstrapNoRequest       errors.Error = "no break-glass request with this id is the newest"
	ErrBootstrapNotPending      errors.Error = "this break-glass request is not pending"
	ErrBootstrapExpired         errors.Error = "this break-glass request passed its %d-hour mark"
	ErrBootstrapNotEligible     errors.Error = "this break-glass request needs %d hours with no bootstrap sign-in, or its approval if a replacement"
	ErrBootstrapEnrolled        errors.Error = "a passkey was enrolled since this request completed: run break-glass to open a new one"
	ErrBootstrapReissueOver     errors.Error = "this request completed more than 24 hours ago: run break-glass to open a new one"
	ErrBootstrapNoTarget        errors.Error = "the account this replacement names doesn't exist: run break-glass with --enroll"
	ErrBootstrapReason          errors.Error = "the reason must be signed-in or denied"
	ErrBootstrapEmailRequired   errors.Error = "email is required"
	ErrBreakGlassWaitInvalid    errors.Error = "%s must be a whole number of hours, at least 1, not %q"
	ErrBootstrapStatusRead      errors.Error = "reading the bootstrap status failed: %v"
	ErrBootstrapStatusWrite     errors.Error = "writing the bootstrap status failed: %v"
	ErrBootstrapStatusInvalid   errors.Error = "the stored bootstrap status is invalid: %v"
	ErrBootstrapWriteContended  errors.Error = "the bootstrap status kept changing: try again"
	ErrBootstrapStatusPruned    errors.Error = "TelarkConfig didn't keep status.bootstrap: upgrade the telark-crds chart first"
	ErrBootstrapMarkerWrite     errors.Error = "setting the bootstrap marker of %s failed: %v"
	ErrBootstrapIdentitiesWrite errors.Error = "removing the passkey sign-ins of %s failed: %v"
	ErrBootstrapPinFailed       errors.Error = "pinning the bootstrap account %s failed, so break-glass refuses to act " +
		"until the exporter pins it at its next start: %v"
	ErrBootstrapRecoveryApproval errors.Error = "a recovery can't be approved: it completes %d hours after it opened " +
		"unless the bootstrap account signs in or denies it"
	ErrBootstrapKeep             errors.Error     = "the passkey to keep isn't the bootstrap account's, registered after this recovery's %d-hour mark"
	ErrBootstrapNoticeAdmins     errors.Error     = "listing the Admins on ALL failed, so only the bootstrap account got this break-glass notice: %v"
	WarnBootstrapMarkersFailed   messages.Message = "clearing the bootstrap marker of other accounts failed: %v"
	WarnBootstrapNoticeFailed    messages.Message = "listing who a break-glass notice goes to failed: %v"
	WarnBootstrapReconcileFailed messages.Message = "[startup] reconciling the bootstrap pin failed, retried at the next start: %v"
	InfBootstrapPinned           messages.Message = "pinned the bootstrap account %s"
	InfBootstrapRequest          messages.Message = "break-glass request %s (%s): %s, cause: %s"
	MsgBootstrapStatusRead       messages.Message = "bootstrap status read"
	MsgBootstrapRequestUpdated   messages.Message = "break-glass request updated"
)

// What a request's log line names: its new state (a canceled one with its reason), or the link break-glass is issuing.
const (
	BootstrapLogCanceled      = "%s (%s)"
	BootstrapLogLinkIssued    = "issuing a link"
	BootstrapLogLinkReissued  = "issuing a link again"
	BootstrapCauseBreakGlass  = "break-glass"
	BootstrapCausePasskey     = "passkey"
	BootstrapCauseSignIn      = "sign-in"
	BootstrapCauseWaitElapsed = "%d hours"
)

const (
	NotifBootstrapOpenedTitle   = "Break-glass request opened"
	NotifBootstrapOpenedRecover = "Someone ran break-glass to recover the bootstrap account. The request completes on its own " +
		"%d hours after it opened, unless the bootstrap account signs in or denies it on the break-glass page first."
	NotifBootstrapOpenedReplace = "Someone ran break-glass to make %s the bootstrap account. "
	NotifBootstrapOpenedMasked  = "Someone ran break-glass to make another account the bootstrap account. "
	NotifBootstrapOpenedRule    = "The request completes when the bootstrap account approves it with a passkey, or " +
		"after %d hours without a sign-in. A sign-in by the bootstrap account cancels it."
	NotifBootstrapCanceledTitle     = "Break-glass request canceled"
	NotifBootstrapCanceledSignedIn  = "The bootstrap account signed in, which canceled the request."
	NotifBootstrapCanceledDenied    = "The bootstrap account denied the request with a passkey."
	NotifBootstrapCanceledExpired   = "The bootstrap account signed in after the request opened, so it ended once its %d hours passed."
	NotifBootstrapCompletedTitle    = "Break-glass request completed"
	NotifBootstrapCompletedRecover  = "Break-glass recovered the bootstrap account: only the passkey its link registered is left, and no session."
	NotifBootstrapCompletedReplaced = "%s is now the bootstrap account."
	NotifBootstrapCompletedMasked   = "Another account is now the bootstrap account."
	NotifBootstrapReissuedTitle     = "Break-glass is issuing a link again"
	NotifBootstrapReissuedMessage   = "Break-glass is printing a new enrollment link for the account a completed replacement made the " +
		"bootstrap account. It can for 24 hours after completion, until that account has a passkey."
	NotifBootstrapIssuedTitle   = "Break-glass is issuing a link"
	NotifBootstrapIssuedMessage = "Break-glass is printing an enrollment link for the bootstrap account, valid 10 minutes. Its passkeys " +
		"stay until a passkey is registered with the link, which then removes every other passkey and session."
)
