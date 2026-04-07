package applications

import "github.com/plsyro/rest/base"

const (
	EnrichApplications      base.Endpoint = "resources/applications/enrich" // namespace, selector params are optional
	CreateApplication       base.Endpoint = "resources/applications/create"
	GetAllApplications      base.Endpoint = "resources/applications/get"
	GetApplicationByName    base.Endpoint = "resources/applications/{name}/get"
	PatchApplicationByName  base.Endpoint = "resources/applications/{name}/patch"
	DeleteApplicationByName base.Endpoint = "resources/applications/{name}/delete"
	SyncApplication         base.Endpoint = "resources/applications/{name}/sync"

	// Application Rollback endpoints
	TriggerRollback base.Endpoint = "resources/applications/{name}/rollbacks"
	GetRollbacks    base.Endpoint = "resources/applications/{name}/rollbacks"
	GetRollback     base.Endpoint = "resources/applications/{name}/rollbacks/{rollbackId}"
)
