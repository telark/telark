package protection

import (
	"github.com/telark/data/errors"
)

const (
	SystemActor           = "system"
	ReasonCanceledByUser  = "Canceled by user."
	ReasonExpired         = "Plan window ended."
	QueryParamLimit       = "limit"
	QueryParamResult      = "result"
	FieldPhase            = "phase"
	FieldReason           = "reason"
	FieldRenderedPolicies = "renderedPolicies"
	FieldTerminatedAt     = "terminatedAt"
	FieldTerminatedBy     = "terminatedBy"
	FieldLastUpdatedAt    = "lastUpdatedAt"
	FieldLastUpdatedBy    = "lastUpdatedBy"
	FieldHealth           = "health"
	FieldStartedAt        = "startedAt"
	FieldStartedBy        = "startedBy"
	LogPlanActivated      = "protection-plan activated plan=%s"
	LogPlanTerminated     = "protection-plan terminated plan=%s"
)

const (
	ErrUserMissing            errors.Error = "X-User-ID header is required"
	ErrMissingApplications    errors.Error = "applications not found: %v"
	ErrCancelInvalidPhase     errors.Error = "plan in phase %q cannot be canceled"
	ErrReactivateInvalidPhase errors.Error = "plan in phase %q cannot be reactivated"
	ErrReactivateExpired      errors.Error = "plan time range has fully elapsed;" +
		" edit the plan with new dates before reactivating"
	ErrReactivateMissingTemplates errors.Error = "policy templates no longer exist: %v"
	ErrRequestBody                errors.Error = "invalid request body: %v"
	ErrIDGeneration               errors.Error = "failed to generate plan id: %v"
	ErrInternal                   errors.Error = "internal error: %v"
	ErrServiceNotReady            errors.Error = "protection plan service is not ready"
	ErrReportBusy                 errors.Error = "a report is already being generated; retry shortly"
)
