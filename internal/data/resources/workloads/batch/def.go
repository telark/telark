package batch

import (
	resourceShared "github.com/plsyro/data-pkg/resources/shared"
	workloadShared "github.com/plsyro/data-pkg/resources/workloads/shared"
)

type BatchWorkloadAsResource struct {
	Fasid  resourceShared.Fasid  `json:"fasid"`
	Cacid  CacidBatch            `json:"cacid"`
	Config resourceShared.Config `json:"config"`
}

type CacidBatch struct {
	Status                 Status                                `json:"status"`
	Metadata               workloadShared.Metadata               `json:"metadata"`
	Parallelism            int                                   `json:"parallelism"`
	Completions            int                                   `json:"completions"`
	RestartPolicy          string                                `json:"restartPolicy"`
	BackoffLimit           int                                   `json:"backoffLimit"`
	Crates                 workloadShared.Crates                 `json:"crates"`
	Registry               workloadShared.RegType                `json:"registry"`
	BridgeAttachmentPolicy workloadShared.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"`
	Events                 []workloadShared.Events               `json:"events"`
	Usage                  workloadShared.Usage                  `json:"usage"`
}

type Status struct {
	Active         int    `json:"active"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	StartTime      string `json:"startTime"`
	CompletionTime string `json:"completionTime"`
}
