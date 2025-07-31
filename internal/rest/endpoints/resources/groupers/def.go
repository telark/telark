package groupers

import "github.com/plsyro/rest-pkg/base"

const (
	CREATE_GROUPER             base.Endpoint = "resources/groupers/create"
	GET_ALL_GROUPERS           base.Endpoint = "resources/groupers/get"
	GET_GROUPER                base.Endpoint = "resources/groupers/{name}/get"
	PATCH_GROUPER              base.Endpoint = "resources/groupers/{name}/patch"
	UPDATE_GROUPER_SYNC        base.Endpoint = "resources/groupers/{name}/update/sync"
	UPDATE_GROUPER_GLOBAL_DATA base.Endpoint = "resources/groupers/{name}/update/global"
	DELETE_GROUPER             base.Endpoint = "resources/groupers/{name}/delete"
)
