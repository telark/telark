package update

import "github.com/telark/data/errors"

const (
	ErrInvalidPhase    errors.Error = "Cannot edit a canceled or terminated plan. Reactivate it first"
	ErrScopeTypeChange errors.Error = "Scope type cannot be changed." +
		" Cancel and recreate the plan instead"
	ErrMissingApplications errors.Error = "applications not found: %v"
	ErrMissingNamespaces   errors.Error = "namespaces not found in cluster: %v"
	ErrInvalidPolicies     errors.Error = "policy validation failed: %v"
	ErrInvalidTimeRange    errors.Error = "timeRange.endAt must be after timeRange.startAt"
	ErrUnknownTemplate     errors.Error = "unknown template id: %s"
	ErrTemplateScope       errors.Error = "template %q does not support scope %q"
	ErrInvalidParams       errors.Error = "template %q params invalid: %v"
	ErrPoliciesRequired    errors.Error = "at least one policy is required"
	ErrPartial             errors.Error = "update partially applied; cluster reverted but" +
		" CRD may be inconsistent — investigate plan %q"
	ErrInternal          errors.Error = "internal error: %v"
	deployFailureGeneric              = "Could not deploy protection rules; please contact your platform administrator"
)
