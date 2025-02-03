package admissions

import "github.com/plsyro/rest-pkg/base"

const (
	// Create A
	CREATE_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/create"

	// Validate Endpoints
	VALIDATE_GROUPER_ADMISSION base.Endpoint = "admissions/groupers/{name}/validate"
)
