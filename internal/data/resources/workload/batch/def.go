package batch

import (
	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
	workloadCommon "github.com/plsyro/data-pkg/resources/workload/common"
)

type BatchWorkloadAsResource struct {
	Fasid  resourcesCommon.Fasid  `json:"fasid"`
	Cacid  CacidBatch             `json:"cacid"`
	Config resourcesCommon.Config `json:"config"`
}

type CacidBatch struct {
	Status                 Status                                `json:"status"`
	Metadata               workloadCommon.Metadata               `json:"metadata"`
	Parallelism            int                                   `json:"parallelism"`
	Completions            int                                   `json:"completions"`
	RestartPolicy          string                                `json:"restartPolicy"`
	BackoffLimit           int                                   `json:"backoffLimit"`
	Crates                 workloadCommon.Crates                 `json:"crates"`
	Registry               workloadCommon.RegType                `json:"registry"`
	BridgeAttachmentPolicy workloadCommon.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"`
	Events                 []workloadCommon.Events               `json:"events"`
	Usage                  workloadCommon.Usage
}

type Status struct {
	Active         int    `json:"active"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	StartTime      string `json:"startTime"`
	CompletionTime string `json:"completionTime"`
}
