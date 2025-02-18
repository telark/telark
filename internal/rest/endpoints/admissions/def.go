package admissions

import "github.com/plsyro/rest-pkg/base"

const (
	// Create Endpoints
	CREATE_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/create"

	// Get Endpoints
	GET_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/{name}/get"

	// Validate Endpoints
	VALIDATE_GROUPER_ADMISSION base.Endpoint = "admissions/groupers/{name}/validate"
)
