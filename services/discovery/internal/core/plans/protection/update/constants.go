package update

import "github.com/telark/data/errors"

const (
	stagePark = "park"

	ErrScopeTypeChange errors.Error = "Scope type cannot be changed." +
		" Cancel and recreate the plan instead"
	ErrMissingApplications errors.Error = "applications not found: %v"
	ErrInvalidPolicies     errors.Error = "policy validation failed: %v"
	ErrUnknownTemplate     errors.Error = "unknown template id: %s"
	ErrTemplateScope       errors.Error = "template %q does not support scope %q"
	ErrInvalidParams       errors.Error = "template %q params invalid: %v"
	ErrPoliciesRequired    errors.Error = "at least one policy is required"
	ErrPartial             errors.Error = "update partially applied; cluster reverted but" +
		" CRD may be inconsistent — investigate plan %q"
	ErrInternal          errors.Error = "internal error: %v"
	deployFailureGeneric              = "Could not deploy protection rules; please contact your platform administrator"
)
