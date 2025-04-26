package groupers

import "github.com/plsyro/rest-pkg/base"

const (
	// Create Endpoints
	CREATE_GROUPER base.Endpoint = "resources/groupers/create"

	// Get Endpoints
	GET_ALL_GROUPERS base.Endpoint = "resources/groupers/get"
	GET_GROUPER      base.Endpoint = "resources/groupers/{name}/get"

	// Update Endpoints
	UPDATE_GROUPER_SYNC        base.Endpoint = "resources/groupers/{name}/update/sync"
	UPDATE_GROUPER_GLOBAL_DATA base.Endpoint = "resources/groupers/{name}/update/global"
)
