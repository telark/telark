package workloads

import "github.com/plsyro/rest/base"

const (
	Createappworkload             base.Endpoint = "resources/workloads/apps/create"
	CreateBatchWorkload           base.Endpoint = "resources/workloads/batches/get"
	GetAllAppsWorkloads           base.Endpoint = "resources/workloads/apps/get"
	GetAllBatchesWorkloads        base.Endpoint = "resources/workloads/batches/get"
	Getappworkload                base.Endpoint = "resources/workloads/apps/{name}/get"
	GetBatchWorkload              base.Endpoint = "resources/workloads/batches/{name}/get"
	UpdateappworkloadSync         base.Endpoint = "resources/workloads/apps/{name}/update/sync"
	UpdateBatchWorkloadSync       base.Endpoint = "resources/workloads/batches/{name}/update/sync"
	UpdateappworkloadGlobalData   base.Endpoint = "resources/workloads/apps/{name}/update/global"
	UpdateBatchWorkloadGlobalData base.Endpoint = "resources/workloads/batches/{name}/update/global"
	Patchappworkload              base.Endpoint = "resources/workloads/apps/{name}/patch"
	PatchBatchWorkload            base.Endpoint = "resources/workloads/batches/{name}/patch"
	Deleteappworkload             base.Endpoint = "resources/workloads/apps/{name}/delete"
	DeleteBatchWorkload           base.Endpoint = "resources/workloads/batches/{name}/delete"
)
