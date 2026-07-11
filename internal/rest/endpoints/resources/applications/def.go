package applications

import "github.com/telark/rest/base"

const (
	// Application endpoints
	EnrichApplications      base.Endpoint = "resources/applications/enrich" // namespace, selector params are optional
	CreateApplication       base.Endpoint = "resources/applications/create"
	GetAllApplications      base.Endpoint = "resources/applications/get"
	GetApplicationByName    base.Endpoint = "resources/applications/{name}/get"
	PatchApplicationByName  base.Endpoint = "resources/applications/{name}/patch"
	DeleteApplicationByName base.Endpoint = "resources/applications/{name}/delete"
	SyncApplication         base.Endpoint = "resources/applications/{name}/sync"
	CleanupApplication      base.Endpoint = "resources/applications/{name}/cleanup"

	// Rollback endpoints
	GetRollbacks    base.Endpoint = "resources/applications/{name}/rollbacks/get"
	GetRollback     base.Endpoint = "resources/applications/{name}/rollbacks/{rollbackId}/get"
	TriggerRollback base.Endpoint = "resources/applications/{name}/rollbacks/trigger"
	AbortRollback   base.Endpoint = "resources/applications/{name}/rollbacks/{rollbackId}/abort"
)
