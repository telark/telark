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
)

const (
	ErrBootstrapPinned          errors.Error = "the bootstrap account is pinned: only a completed break-glass replacement moves the marker"
	ErrBootstrapNotPinned       errors.Error = "no bootstrap account is pinned yet, so break-glass acts at once"
	ErrBootstrapNoRequest       errors.Error = "no break-glass request with this id is the newest"
	ErrBootstrapNotPending      errors.Error = "this break-glass request is not pending"
	ErrBootstrapExpired         errors.Error = "this break-glass request passed its 72-hour mark"
	ErrBootstrapNotEligible     errors.Error = "this break-glass request needs the bootstrap account's approval, or 72 hours with no sign-in"
	ErrBootstrapEnrolled        errors.Error = "a passkey was enrolled since this request completed: run break-glass to open a new one"
	ErrBootstrapReissueOver     errors.Error = "this request completed more than 24 hours ago: run break-glass to open a new one"
	ErrBootstrapNoTarget        errors.Error = "the account this replacement names doesn't exist: run break-glass with --enroll"
	ErrBootstrapReason          errors.Error = "the reason must be signed-in or denied"
	ErrBootstrapEmailRequired   errors.Error = "email is required"
	ErrBootstrapStatusRead      errors.Error = "reading the bootstrap status failed: %v"
	ErrBootstrapStatusWrite     errors.Error = "writing the bootstrap status failed: %v"
	ErrBootstrapStatusInvalid   errors.Error = "the stored bootstrap status is invalid: %v"
	ErrBootstrapWriteContended  errors.Error = "the bootstrap status kept changing: try again"
	ErrBootstrapStatusPruned    errors.Error = "TelarkConfig didn't keep status.bootstrap: upgrade the telark-crds chart first"
	ErrBootstrapMarkerWrite     errors.Error = "setting the bootstrap marker of %s failed: %v"
	ErrBootstrapIdentitiesWrite errors.Error = "removing the passkey sign-ins of %s failed: %v"
	ErrBootstrapPinFailed       errors.Error = "pinning the bootstrap account %s failed, so break-glass refuses to act " +
		"until the exporter pins it at its next start: %v"
	WarnBootstrapMarkersFailed   messages.Message = "clearing the bootstrap marker of other accounts failed: %v"
	WarnBootstrapNoticeFailed    messages.Message = "listing who a break-glass notice goes to failed: %v"
	WarnBootstrapReconcileFailed messages.Message = "[startup] reconciling the bootstrap pin failed, retried at the next start: %v"
	InfBootstrapPinned           messages.Message = "pinned the bootstrap account %s"
	MsgBootstrapStatusRead       messages.Message = "bootstrap status read"
	MsgBootstrapRequestUpdated   messages.Message = "break-glass request updated"
)

const (
	NotifBootstrapOpenedTitle   = "Break-glass request opened"
	NotifBootstrapOpenedRecover = "Someone ran break-glass to recover the bootstrap account. "
	NotifBootstrapOpenedReplace = "Someone ran break-glass to make %s the bootstrap account. "
	NotifBootstrapOpenedMasked  = "Someone ran break-glass to make another account the bootstrap account. "
	NotifBootstrapOpenedRule    = "The request completes when the bootstrap account approves it with a passkey, or " +
		"after 72 hours without a sign-in. A sign-in by the bootstrap account cancels it."
	NotifBootstrapCanceledTitle     = "Break-glass request canceled"
	NotifBootstrapCanceledSignedIn  = "The bootstrap account signed in, which canceled the request."
	NotifBootstrapCanceledDenied    = "The bootstrap account denied the request with a passkey."
	NotifBootstrapCanceledExpired   = "The bootstrap account signed in after the request opened, so it ended once its 72 hours passed."
	NotifBootstrapCompletedTitle    = "Break-glass request completed"
	NotifBootstrapCompletedRecover  = "The bootstrap account's passkeys and sessions were removed. A new passkey can now be enrolled on it."
	NotifBootstrapCompletedReplaced = "%s is now the bootstrap account."
	NotifBootstrapCompletedMasked   = "Another account is now the bootstrap account."
	NotifBootstrapReissuedTitle     = "Break-glass link issued again"
	NotifBootstrapReissuedMessage   = "Break-glass printed a new enrollment link for the bootstrap account, whose recovery " +
		"completed. It can for 24 hours after completion, until a passkey is enrolled."
)
