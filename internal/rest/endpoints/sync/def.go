package sync

import "github.com/plsyro/rest/base"

const (
	SyncAllGroupers         base.Endpoint = "resources/groupers/sync"
	SyncGrouper             base.Endpoint = "resources/groupers/{name}/sync"
	SyncAllAppsWorkloads    base.Endpoint = "resources/workloads/apps/sync"
	Syncappworkload         base.Endpoint = "resources/workloads/apps/{name}/sync"
	SyncAllBatchesWorkloads base.Endpoint = "resources/workloads/batches/sync"
	SyncBatchWorkload       base.Endpoint = "resources/workloads/batches/{name}/sync"
	SyncAllBridges          base.Endpoint = "resources/bridges/sync"
	SyncBridge              base.Endpoint = "resources/bridges/{name}/sync"
)
