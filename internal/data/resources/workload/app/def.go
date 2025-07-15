package app

import (
	"github.com/plsyro/data-pkg/resources/common"
	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
	workloadCommon "github.com/plsyro/data-pkg/resources/workload/common"
)

type AppWorkloadAsResource struct {
	Fasid  resourcesCommon.Fasid `json:"fasid"`
	Cacid  CacidApp              `json:"cacid"`
	Config common.Config         `json:"config"`
}

type CacidApp struct {
	Status                 string                                `json:"status"`
	Metadata               workloadCommon.Metadata               `json:"metadata"`
	Strategy               string                                `json:"strategy"`
	Instances              workloadCommon.Instances              `json:"instances"`
	Crates                 workloadCommon.Crates                 `json:"crates"`
	Registry               workloadCommon.RegType                `json:"registry"`
	Bridges                []Bridge                              `json:"bridges"`
	BridgeAttachmentPolicy workloadCommon.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"`
	Events                 []workloadCommon.Events               `json:"events"`
}

type Bridge struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsSameGrouper bool   `json:"isSameGrouper"`
}
