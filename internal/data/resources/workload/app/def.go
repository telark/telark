package app

import (
	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
	workloadCommon "github.com/plsyro/data-pkg/resources/workload/common"
)

type AppWorkloadAsResource struct {
	Fasid resourcesCommon.Fasid `json:"fasid"`
	Cacid CacidApp              `json:"cacid"`
}

type CacidApp struct {
	Status                 string                                `json:"status"`
	Metadata               workloadCommon.Metadata               `json:"metadata"`
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
	Type          string `json:"type"`
	IsSameGrouper bool   `json:"isSameGrouper"`
}
