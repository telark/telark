package admissions

import "github.com/plsyro/rest-pkg/base"

const (
	CREATE_ADMISSION_VALIDATING_WEBHOOK base.Endpoint = "admissions/validation/create"
	GET_ADMISSION_VALIDATING_WEBHOOK    base.Endpoint = "admissions/validation/{name}/get"
	PATCH_ADMISSION_VALIDATING_WEBHOOK  base.Endpoint = "admissions/validation/{name}/patch"
	DELETE_ADMISSION_VALIDATING_WEBHOOK base.Endpoint = "admissions/validation/{name}/delete"
	VALIDATE_GROUPER                    base.Endpoint = "admissions/validation/groupers/{name}/validate"
)
