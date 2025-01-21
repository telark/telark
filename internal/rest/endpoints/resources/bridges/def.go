package resources

import (
	"github.com/plsyro/rest-pkg/base"
)

const (
	// Create Endpoints
	CREATE_BRIDGE base.Endpoint = "resources/bridges/create"

	// Get Endpoints
	GET_ALL_BRIDGES base.Endpoint = "resources/bridges/get"
	GET_BRIDGE      base.Endpoint = "resources/bridges/{name}/get"

	// Update Endpoints
	UPDATE_BRIDGE_SYNC        base.Endpoint = "resources/bridges/{name}/update/sync"
	UPDATE_BRIDGE_GLOBAL_DATA base.Endpoint = "resources/bridges/{name}/update/global"
)
