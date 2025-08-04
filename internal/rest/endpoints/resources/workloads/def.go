package workloads

import "github.com/plsyro/rest-pkg/base"

const (
	CreateAppWorkload             base.Endpoint = "resources/workloads/apps/create"
	CreateBatchWorkload           base.Endpoint = "resources/workloads/batches/get"
	GetAllAppsWorkloads           base.Endpoint = "resources/workloads/apps/get"
	GetAllBatchesWorkloads        base.Endpoint = "resources/workloads/batches/get"
	GetAppWorkload                base.Endpoint = "resources/workloads/apps/{name}/get"
	GetBatchWorkload              base.Endpoint = "resources/workloads/batches/{name}/get"
	UpdateAppWorkloadSync         base.Endpoint = "resources/workloads/apps/{name}/update/sync"
	UpdateBatchWorkloadSync       base.Endpoint = "resources/workloads/batches/{name}/update/sync"
	UpdateAppWorkloadGlobalData   base.Endpoint = "resources/workloads/apps/{name}/update/global"
	UpdateBatchWorkloadGlobalData base.Endpoint = "resources/workloads/batches/{name}/update/global"
	PatchAppWorkload              base.Endpoint = "resources/workloads/apps/{name}/patch"
	PatchBatchWorkload            base.Endpoint = "resources/workloads/batches/{name}/patch"
	DeleteAppWorkload             base.Endpoint = "resources/workloads/apps/{name}/delete"
	DeleteBatchWorkload           base.Endpoint = "resources/workloads/batches/{name}/delete"
)
