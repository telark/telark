package batch

import (
	resourceshared "github.com/plsyro/data/resources/shared"
	workloadshared "github.com/plsyro/data/resources/workloads/shared"
)

type BatchWorkloadAsResource struct {
	Fasid  resourceshared.Fasid  `json:"fasid"`
	Cacid  CacidBatch            `json:"cacid"`
	Config resourceshared.Config `json:"config"`
}

type CacidBatch struct {
	Status                 Status                                `json:"status"`
	Metadata               workloadshared.Metadata               `json:"metadata"`
	Parallelism            int                                   `json:"parallelism"`
	Completions            int                                   `json:"completions"`
	RestartPolicy          string                                `json:"restartPolicy"`
	BackoffLimit           int                                   `json:"backoffLimit"`
	Crates                 workloadshared.Crates                 `json:"crates"`
	Registry               workloadshared.RegType                `json:"registry"`
	BridgeAttachmentPolicy workloadshared.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"`
	Events                 []workloadshared.Events               `json:"events"`
	Usage                  workloadshared.Usage                  `json:"usage"`
}

type Status struct {
	Active         int    `json:"active"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	StartTime      string `json:"startTime"`
	CompletionTime string `json:"completionTime"`
}
