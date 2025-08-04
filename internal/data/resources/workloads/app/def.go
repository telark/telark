package app

import (
	resourceShared "github.com/plsyro/data-pkg/resources/shared"
	workloadShared "github.com/plsyro/data-pkg/resources/workloads/shared"
)

type AppWorkloadAsResource struct {
	Fasid  resourceShared.Fasid  `json:"fasid"`
	Cacid  CacidApp              `json:"cacid"`
	Config resourceShared.Config `json:"config"`
}

type CacidApp struct {
	Status                 string                                `json:"status"`
	Metadata               workloadShared.Metadata               `json:"metadata"`
	Strategy               string                                `json:"strategy"`
	Instances              workloadShared.Instances              `json:"instances"`
	Crates                 workloadShared.Crates                 `json:"crates"`
	Registry               workloadShared.RegType                `json:"registry"`
	Bridges                []Bridge                              `json:"bridges"`
	BridgeAttachmentPolicy workloadShared.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"`
	Events                 []workloadShared.Events               `json:"events"`
	Usage                  workloadShared.Usage                  `json:"usage"`
}

type Bridge struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsSameGrouper bool   `json:"isSameGrouper"`
}
