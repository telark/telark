package constants

import (
	"time"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
)

const (
	LoggerPrefixAuthRequest  = "auth-request"
	AuthRequestPollInterval  = 5 * time.Second
	AuthRequestKeyBytes      = 32
	AuthRequestEnrollURL     = "%s/register#enroll=%s&email=%s"
	AuthRequestApprovalURL   = "%s/auth/request#request=%s&kind=%s"
	AuthRequestDashboardURL  = "<dashboard-url>"
	AuthRequestKindRecover   = "recover %s"
	AuthRequestKindReplace   = "make %s the bootstrap account"
	AuthRequestCreateStatus  = "status %d: %s"
	AuthRequestNotCreated    = "it was not found once created"
	authRequestCheckExporter = ". Check that the exporter is running, then run telark auth enroll again"
)

// What a request's status says, one sentence per phase.
const (
	AuthRequestNotBootstrap = "%s is not the bootstrap admin email (BOOTSTRAP_ADMIN)"
	AuthRequestOtherRequest = "another request is open: %s (%s). Run telark auth enroll again once it completes, " +
		"or the bootstrap account denies it or signs in"
	AuthRequestWaitReplace = "request %s is open: %s. Run telark auth enroll again after completesAt, " +
		"or once the bootstrap account approves it at approvalUrl"
	AuthRequestWaitRecover = "request %s is open: %s. Run telark auth enroll again after completesAt. " +
		"The bootstrap account can deny it until then at approvalUrl"
	AuthRequestRecovering  = "ending the sessions of %s and revoking what was saved after the wait, before the link"
	AuthRequestLinkReady   = "register a passkey for %s with the link, in a browser that is not signed in to Telark"
	AuthRequestEnrolled    = "%s registered a passkey with the link"
	AuthRequestRecovered   = "request %s completed: %s now has only the new passkey"
	AuthRequestReplaced    = "request %s completed: %s is now the bootstrap account and already has a passkey. Sign in with it"
	AuthRequestLinkUnused  = "the link expired unused. Run telark auth enroll again for a new link"
	AuthRequestLinkExpired = "the link expired unused: nothing changed and request %s stays open. " +
		"Run telark auth enroll again for a new link"
	AuthRequestEnded = "request %s was canceled by a sign-in or a deny: %s keeps its passkeys. " +
		"Run telark auth enroll again to open a new request"
	AuthRequestInterrupted = "auth stopped while this request ran. Run telark auth enroll again"
	AuthRequestPinMissing  = "nothing changed: %s has a passkey but is not pinned. Upgrade telark-crds if it is older " +
		"than the app chart, restart the exporter, then run telark auth enroll again"
)

const (
	ErrAuthRequestRecipientKey   errors.Error     = "recipientKey is not a base64 X25519 public key"
	ErrAuthRequestStatusFailed   errors.Error     = "could not read the bootstrap account: %v" + authRequestCheckExporter
	ErrAuthRequestLookupFailed   errors.Error     = "could not look up %s: %v" + authRequestCheckExporter
	ErrAuthRequestCreateFailed   errors.Error     = "could not create %s: %v"
	ErrAuthRequestPatchFailed    errors.Error     = "could not promote %s: status %d: %s"
	ErrAuthRequestLinkFailed     errors.Error     = "could not create the enrollment link: %v"
	ErrAuthRequestOpenFailed     errors.Error     = "could not open a request: %v"
	ErrAuthRequestCompleteFailed errors.Error     = "could not finish request %s: %v"
	ErrAuthRequestRunFailed      errors.Error     = "could not run the auth request: %v"
	ErrAuthRequestReportFailed   errors.Error     = "auth request %s: reporting %s failed: status %d: %s"
	ErrAuthRequestStepFailed     errors.Error     = "auth request %s: %s"
	InfAuthRequestCreated        messages.Message = "auth request %s: created %s with the Admin role"
	InfAuthRequestPromoted       messages.Message = "auth request %s: %s is now the bootstrap admin, with the Admin role"
)
