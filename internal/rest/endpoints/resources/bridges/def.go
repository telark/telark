package resources

import "github.com/plsyro/rest-pkg/base"

const (
	CREATE_BRIDGE             base.Endpoint = "resources/bridges/create"
	GET_ALL_BRIDGES           base.Endpoint = "resources/bridges/get"
	GET_BRIDGE                base.Endpoint = "resources/bridges/{name}/get"
	PATCH_BRIDGE              base.Endpoint = "resources/bridges/{name}/patch"
	UPDATE_BRIDGE_SYNC        base.Endpoint = "resources/bridges/{name}/update/sync"
	UPDATE_BRIDGE_GLOBAL_DATA base.Endpoint = "resources/bridges/{name}/update/global"
	DELETE_BRIDGE             base.Endpoint = "resources/bridges/{name}/delete"
)
