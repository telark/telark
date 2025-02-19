package admissions

import "github.com/plsyro/rest-pkg/base"

const (
	// Create Endpoints
	CREATE_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/create"

	// Get Endpoints
	GET_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/{name}/get"

	// Patch Endpoints
	PATCH_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/{name}/patch"

	// Delete Endpoints
	DELETE_ADMISSION_VALIDATION_WEBHOOK base.Endpoint = "admissions/validation/{name}/delete"

	// Validate Endpoints
	VALIDATE_GROUPER base.Endpoint = "admissions/validation/groupers/{name}/validate"
)
