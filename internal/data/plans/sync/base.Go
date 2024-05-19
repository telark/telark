package sync

import "github.com/plsyro/common-pkg/v2/data/common"

type SyncPlan struct {
	PlanGlobalData PlanGlobalData `json:"planGlobalData"`
	PlanCustomData PlanCustomData `json:"planCustomData"`
}

type PlanGlobalData struct {
	CreationTime common.OperationTime `json:"creationTime"`
	UpdateTime   common.OperationTime `json:"updateTime"`
}

type PlanCustomData struct {
	PlanType    string `json:"planType"`
	Mode        string `json:"mode"`
	Locked      bool   `json:"locked"`
	Parallelism int    `json:"parallelism"`
	Frequency   int    `json:"frequency"`
	Batch       int    `json:"batch"`
	Total       int    `json:"total"`
}
