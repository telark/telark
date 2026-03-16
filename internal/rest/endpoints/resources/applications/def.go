package applications

import "github.com/plsyro/rest/base"

const (
	CreateApplication     base.Endpoint = "resources/applications/create"
	GetAllApplications    base.Endpoint = "resources/applications/get"
	GetApplicationByID    base.Endpoint = "resources/applications/{id}/get"
	PatchApplicationByID  base.Endpoint = "resources/applications/{id}/patch"
	DeleteApplicationByID base.Endpoint = "resources/applications/{id}/delete"
)
