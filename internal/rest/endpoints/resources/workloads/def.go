package workloads

import "github.com/plsyro/rest-pkg/base"

const (
	// Create Endpoints
	CREATE_APP_WORKLOAD   base.Endpoint = "resources/workloads/apps/create"
	CREATE_BATCH_WORKLOAD base.Endpoint = "resources/workloads/batches/create"

	// Get Endpoints
	GET_ALL_APPS_WORKLOADS    base.Endpoint = "resources/workloads/apps/get"
	GET_ALL_BATCHES_WORKLOADS base.Endpoint = "resources/workloads/batches/get"
	GET_APP_WORKLOAD          base.Endpoint = "resources/workloads/apps/{name}/get"
	GET_BATCH_WORKLOAD        base.Endpoint = "resources/workloads/batches/{name}/get"

	// Update Endpoints
	UPDATE_APP_WORKLOAD_SYNC          base.Endpoint = "resources/workloads/apps/{name}/update/sync"
	UPDATE_BATCH_WORKLOAD_SYNC        base.Endpoint = "resources/workloads/batches/{name}/update/sync"
	UPDATE_APP_WORKLOAD_GLOBAL_DATA   base.Endpoint = "resources/workloads/apps/{name}/update/global"
	UPDATE_BATCH_WORKLOAD_GLOBAL_DATA base.Endpoint = "resources/workloads/batches/{name}/update/global"
)
