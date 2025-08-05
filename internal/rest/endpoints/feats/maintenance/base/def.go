package base

import "github.com/plsyro/rest/base"

const (
	CreateMaintenanceFeat base.Endpoint = "feats/maintenance/create"
	GetMaintenanceFeat    base.Endpoint = "feats/maintenance/{name}/get"
	PatchMaintenanceFeat  base.Endpoint = "feats/maintenance/{name}/patch"
	DeleteMaintenanceFeat base.Endpoint = "feats/maintenance/{name}/delete"
)
