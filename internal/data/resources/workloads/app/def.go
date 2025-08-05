package app

import (
	resourceshared "github.com/plsyro/data/resources/shared"
	workloadshared "github.com/plsyro/data/resources/workloads/shared"
)

type AppWorkloadAsResource struct {
	Fasid  resourceshared.Fasid  `json:"fasid"`
	Cacid  CacidApp              `json:"cacid"`
	Config resourceshared.Config `json:"config"`
}

type CacidApp struct {
	Status                 string                                `json:"status"`
	Metadata               workloadshared.Metadata               `json:"metadata"`
	Strategy               string                                `json:"strategy"`
	Instances              workloadshared.Instances              `json:"instances"`
	Crates                 workloadshared.Crates                 `json:"crates"`
	Registry               workloadshared.RegType                `json:"registry"`
	Bridges                []Bridge                              `json:"bridges"`
	BridgeAttachmentPolicy workloadshared.BridgeAttachmentPolicy `json:"bridgeAttachmentPolicy"` //nolint:revive
	Events                 []workloadshared.Events               `json:"events"`
	Usage                  workloadshared.Usage                  `json:"usage"`
}

type Bridge struct {
	Name          string `json:"name"`
	Type          string `json:"type"`
	IsSameGrouper bool   `json:"isSameGrouper"`
}
