package admissions

import "github.com/plsyro/rest/base"

const (
	CreateAdmissionValidatingWebhook base.Endpoint = "admissions/validation/create"
	GetAdmissionValidatingWebhook    base.Endpoint = "admissions/validation/{name}/get"
	PatchAdmissionValidatingWebhook  base.Endpoint = "admissions/validation/{name}/patch"
	DeleteAdmissionValidatingWebhook base.Endpoint = "admissions/validation/{name}/delete"
	ValidateGrouper                  base.Endpoint = "admissions/validation/groupers/{name}/validate" //nolint:revive
)
