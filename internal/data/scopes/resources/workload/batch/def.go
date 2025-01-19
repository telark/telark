package batch

import (
	allScopesCommon "github.com/plsyro/scopes-pkg/scopes/resources/common"
	workloadCommon "github.com/plsyro/scopes-pkg/scopes/resources/workload/common"
)

type BatchWorkloadAsResource struct {
	Fasid allScopesCommon.Fasid `json:"fasid"`
	Cacid CacidBatch            `json:"cacid"`
}

type CacidBatch struct {
	Status                 Status                                `json:"status"`
	Metadata               workloadCommon.Metadata               `json:"metadata"`
	Kind                   string                                `json:"kind"`
	Parallelism            int                                   `json:"parallelism"`
	Completions            int                                   `json:"completions"`
	RestartPolicy          string                                `json:"restartPolicy"`
	BackoffLimit           int                                   `json:"backoffLimit"`
	Crates                 workloadCommon.Crates                 `json:"crates"`
	Registry               string                                `json:"registry"`
	BridgeAttachmentPolicy workloadCommon.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy,omitempty"`
	Events                 []workloadCommon.Events               `json:"events"`
}

type Status struct {
	Active         int    `json:"active"`
	Succeeded      int    `json:"succeeded"`
	Failed         int    `json:"failed"`
	StartTime      string `json:"startTime"`
	CompletionTime string `json:"completionTime"`
}
