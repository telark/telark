package base

import "github.com/plsyro/rest-pkg/base"

const (
	CREATE_MAINTENANCE_FEAT base.Endpoint = "feats/maintenance/create"
	GET_MAINTENANCE_FEAT    base.Endpoint = "feats/maintenance/{name}/get"
	PATCH_MAINTENANCE_FEAT  base.Endpoint = "feats/maintenance/{name}/patch"
	DELETE_MAINTENANCE_FEAT base.Endpoint = "feats/maintenance/{name}/delete"
)
