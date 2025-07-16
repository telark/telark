package sync

import "github.com/plsyro/rest-pkg/base"

const (
	// Groupers
	SYNC_ALL_GROUPERS base.Endpoint = "resources/groupers/sync"
	SYNC_GROUPER     base.Endpoint = "resources/groupers/{name}/sync"

	// App Workloads
	SYNC_ALL_APPS_WORKLOADS    base.Endpoint = "resources/workloads/apps/sync"
	SYNC_APP_WORKLOAD          base.Endpoint = "resources/workloads/apps/{name}/sync"
	
	// Batch Workloads
	SYNC_ALL_BATCHES_WORKLOADS base.Endpoint = "resources/workloads/batches/sync"
	SYNC_BATCH_WORKLOAD        base.Endpoint = "resources/workloads/batches/{name}/sync"

	// Bridges
	SYNC_ALL_BRIDGES base.Endpoint = "resources/bridges/sync"
	SYNC_BRIDGE     base.Endpoint = "resources/bridges/{name}/sync"
)