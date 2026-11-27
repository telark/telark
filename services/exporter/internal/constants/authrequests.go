package constants

import (
	"time"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
)

const (
	AuthRequestRetentionDaysEnv     = "AUTH_REQUEST_RETENTION_DAYS"
	DefaultAuthRequestRetentionDays = 30
	AuthRequestCleanupInterval      = time.Hour
	AuthRequestDay                  = 24 * time.Hour
	PrefixAuthRequests              = "AuthRequests: "
)

const (
	ErrAuthRequestNotFound            errors.Error     = "no auth request has this name"
	ErrAuthRequestPhase               errors.Error     = "the phase must be one an auth request reports"
	ErrAuthRequestList                errors.Error     = "listing the auth requests failed: %v"
	ErrAuthRequestStatusWrite         errors.Error     = "writing the status of auth request %s failed: %v"
	ErrAuthRequestStatusPruned        errors.Error     = "auth request %s didn't keep its status: upgrade the telark-crds chart first"
	ErrAuthRequestDeleteFailed        errors.Error     = "deleting auth request %s failed: %v"
	ErrAuthRequestInformerStartFailed errors.Error     = "auth request informer start failed: %v"
	ErrAuthRequestRetentionInvalid    errors.Error     = "%s must be a whole number of days, at least 1, not %q"
	WarnAuthRequestInformerNotSynced  messages.Message = "auth request informer not synced after %s: pending requests are listed from the apiserver"
	InfAuthRequestInformerSynced      messages.Message = "auth request informer synced: %d requests in %s"
	InfAuthRequestStatus              messages.Message = "auth request %s for %s, by %s: %s"
	InfAuthRequestsCleaned            messages.Message = "deleted %d auth requests older than %d days"
	MsgAuthRequestsListed             messages.Message = "pending auth requests listed"
	MsgAuthRequestStatusUpdated       messages.Message = "auth request status updated"
)
