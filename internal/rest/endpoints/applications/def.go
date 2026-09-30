package applications

import "github.com/telark/rest/base"

const (
	CreateApplication       base.Endpoint = "applications"
	GetAllApplications      base.Endpoint = "applications"
	GetApplicationByName    base.Endpoint = "applications/{name}"
	PatchApplicationByName  base.Endpoint = "applications/{name}"
	DeleteApplicationByName base.Endpoint = "applications/{name}"
	SyncApplication         base.Endpoint = "applications/{name}/sync"
	ResetApplication        base.Endpoint = "applications/{name}/reset"
	DiscoveryStatus         base.Endpoint = "discovery/status"

	GetRollbacks    base.Endpoint = "applications/{name}/rollbacks"
	GetRollback     base.Endpoint = "applications/{name}/rollbacks/{rollbackId}"
	TriggerRollback base.Endpoint = "applications/{name}/rollbacks"
	AbortRollback   base.Endpoint = "applications/{name}/rollbacks/{rollbackId}/abort"
)
