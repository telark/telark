package applications

import "github.com/plsyro/rest/base"

const (
	CreateApplication     base.Endpoint = "resources/applications/create"
	GetAllApplications    base.Endpoint = "resources/applications/get"
	GetApplicationByName    base.Endpoint = "resources/applications/{name}/get"
	PatchApplicationByName  base.Endpoint = "resources/applications/{name}/patch"
	DeleteApplicationByName base.Endpoint = "resources/applications/{name}/delete"
)
