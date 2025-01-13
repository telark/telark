package app

import (
	allScopesCommon "github.com/plsyro/scopes-pkg/scopes/common"
	workloadCommon "github.com/plsyro/scopes-pkg/scopes/workload/common"
)

type ResourceAsAppWorkload struct {
	Fasid allScopesCommon.Fasid `json:"fasid"`
	Cacid CacidApp              `json:"cacid"`
}

type CacidApp struct {
	Status                 string                                `json:"status"`
	Metadata               workloadCommon.Metadata               `json:"metadata"`
	Kind                   string                                `json:"kind"`
	Strategy               string                                `json:"strategy,omitempty"`
	Instances              workloadCommon.Instances              `json:"instances"`
	Crates                 workloadCommon.Crates                 `json:"crates"`
	Registry               string                                `json:"registry"`
	Bridges                []Bridge                              `json:"bridges,omitempty"`
	BridgeAttachmentPolicy workloadCommon.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy,omitempty"`
	Events                 []workloadCommon.Events               `json:"events"`
}

type Bridge struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	IsSameGrouper bool   `json:"isSameGrouper"`
}
